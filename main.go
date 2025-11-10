package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // permite conexões de qualquer origem
	},
}

// Variáveis globais para gerenciar clientes conectados
var (
	clients     = make(map[*websocket.Conn]bool)
	clientsMu   sync.Mutex
	lastReading *SensorData
)

const (
	maxRequestBodyBytes = 8 * 1024
	maxLoggedBodyBytes  = 1024
)

func sanitizeForLog(content []byte) string {
	for i, b := range content {
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			content[i] = '.'
		}
	}
	return string(content)
}

func formatFloatWithUnit(value *float64, unit string) string {
	if value == nil {
		return "dado não recebido"
	}
	return fmt.Sprintf("%.1f%s", *value, unit)
}

func formatIntWithUnit(value *int, unit string) string {
	if value == nil {
		return "dado não recebido"
	}
	return fmt.Sprintf("%d%s", *value, unit)
}

type SensorData struct {
	Tipo            string   `json:"tipo"`               // Ex.: "leituras"
	DataHora        string   `json:"data_hora"`          // Formato: "DD/MM/YYYY HH:MM:SS"
	Temperatura     *float64 `json:"temperatura"`        // Temperatura em °C (pode não ser enviada)
	UmidadeAr       *float64 `json:"umidade_ar"`         // Umidade do ar em %
	Luminosidade    *int     `json:"luminosidade"`       // Luminosidade 0-100
	UmidadeSolo     *int     `json:"umidade_solo"`       // Umidade do solo calibrada em %
	SoloBruto       *int     `json:"solo_bruto"`         // Valor bruto do sensor ADC
	SoloBrutoLegacy *int     `json:"umidade_solo_bruto"` // Payload legado do bridge
	StatusBomba     string   `json:"status_bomba"`       // "Bomba ativada" ou "Bomba desativada"
	StatusLuz       string   `json:"status_luz"`         // "Luz ligada" ou "Luz desligada"
}

func main() {
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/api/sensor/latest", handleLatestReading)
	http.HandleFunc("/api/sensor/push", handleSensorPush)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Porta configurável para deploy (Render, Heroku, etc)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // fallback para desenvolvimento local
	}

	log.Println("🚀 Servidor WebSocket rodando em :" + port + "/ws")
	log.Println("📊 API REST disponível em :" + port + "/api/sensor/latest")
	log.Println("🌱 Endpoint para estufa em :" + port + "/api/sensor/push")
	log.Println("💚 Health check em :" + port + "/health")

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("❌ Erro no servidor:", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("❌ Erro ao atualizar conexão:", err)
		return
	}
	defer conn.Close()

	remoteHost := r.RemoteAddr
	if host, _, errSplit := net.SplitHostPort(remoteHost); errSplit == nil {
		remoteHost = host
	}
	userAgent := r.Header.Get("User-Agent")
	if userAgent == "" {
		userAgent = "desconhecido"
	}

	// Registra o novo cliente
	clientsMu.Lock()
	clients[conn] = true
	activeConnections := len(clients)
	clientsMu.Unlock()

	log.Printf("🔗 Sessão WebSocket estabelecida | origem=%s | ua=%s | conexões=%d", remoteHost, userAgent, activeConnections)

	// Se existe última leitura, envia imediatamente
	if lastReading != nil {
		jsonData, _ := json.Marshal(lastReading)
		conn.WriteMessage(websocket.TextMessage, jsonData)
		log.Printf("↩️ Última leitura replicada para sessão recente | origem=%s", remoteHost)
	}

	// Mantém conexão aberta e aguarda desconexão
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("⚠️ Sessão WebSocket encerrada com erro | origem=%s | detalhe=%v", remoteHost, err)
			break
		}
	}

	// Remove cliente quando desconectar
	clientsMu.Lock()
	delete(clients, conn)
	activeConnections = len(clients)
	clientsMu.Unlock()

	log.Printf("📴 Sessão WebSocket finalizada | origem=%s | conexões_ativas=%d", remoteHost, activeConnections)
}

func handleLatestReading(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if lastReading == nil {
		w.WriteHeader(http.StatusNoContent)
		json.NewEncoder(w).Encode(map[string]string{"message": "Nenhuma leitura disponível ainda"})
		return
	}

	json.NewEncoder(w).Encode(lastReading)
}

// Endpoint que a ESTUFA vai chamar para enviar dados
func handleSensorPush(w http.ResponseWriter, r *http.Request) {
	// Headers CORS (para aceitar de qualquer origem)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Responde preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Só aceita POST
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes))
	if err != nil {
		log.Printf("❌ Erro ao ler body da requisição (%s): %v", r.RemoteAddr, err)
		http.Error(w, "Erro ao ler requisição", http.StatusBadRequest)
		return
	}

	if len(bodyBytes) == 0 {
		log.Printf("⚠️ Requisição vazia recebida da estufa (%s)", r.RemoteAddr)
		http.Error(w, "Body vazio", http.StatusBadRequest)
		return
	}

	var data SensorData
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		logBody := string(bodyBytes)
		if len(logBody) > maxLoggedBodyBytes {
			logBody = logBody[:maxLoggedBodyBytes] + "...(truncado)"
		}
		log.Printf("❌ JSON inválido recebido (%s): %v | Payload=%s", r.RemoteAddr, err, logBody)
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Normalização para payloads legados que usam outras chaves
	if data.SoloBruto == nil && data.SoloBrutoLegacy != nil && *data.SoloBrutoLegacy > 0 {
		data.SoloBruto = data.SoloBrutoLegacy
		data.SoloBrutoLegacy = nil
	}
	if data.Tipo == "" {
		data.Tipo = "dado não recebido"
	}
	if data.DataHora == "" {
		data.DataHora = "dado não recebido"
	}
	if data.StatusBomba == "" {
		data.StatusBomba = "dado não recebido"
	}
	if data.StatusLuz == "" {
		data.StatusLuz = "dado não recebido"
	}

	log.Printf("📥 Payload recebido da estufa (%s): %s", r.RemoteAddr, sanitizeForLog(bodyBytes))

	// Armazena como última leitura
	lastReading = &data

	log.Printf("🌱 Recebido da estufa: Tipo=%s, Data=%s, Temp=%s, Umidade=%s, Luz=%s, Solo=%s (Bruto=%s), StatusBomba=%s, StatusLuz=%s",
		data.Tipo,
		data.DataHora,
		formatFloatWithUnit(data.Temperatura, "°C"),
		formatFloatWithUnit(data.UmidadeAr, "%"),
		formatIntWithUnit(data.Luminosidade, "%"),
		formatIntWithUnit(data.UmidadeSolo, "%"),
		formatIntWithUnit(data.SoloBruto, ""),
		data.StatusBomba,
		data.StatusLuz)

	// Envia para todos os dashboards conectados via WebSocket
	broadcastToClients(data)

	// Responde sucesso para a estufa
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "received",
		"message": "Dados recebidos com sucesso",
	})
}

// Função para enviar dados para todos os clientes WebSocket
func broadcastToClients(data SensorData) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Println("❌ Erro ao serializar JSON:", err)
		return
	}

	clientsMu.Lock()
	defer clientsMu.Unlock()

	for client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, jsonData); err != nil {
			log.Println("⚠️ Erro ao enviar para cliente, removendo:", err)
			client.Close()
			delete(clients, client)
		}
	}

	if len(clients) > 0 {
		log.Printf("📡 Broadcast enviado para %d cliente(s)", len(clients))
	}
}
