package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

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

type SensorData struct {
	DataHora         string  `json:"data_hora"`          // Formato: "DD/MM/YYYY HH:MM:SS"
	Temperatura      float64 `json:"temperatura"`        // Temperatura em °C
	UmidadeAr        float64 `json:"umidade_ar"`         // Umidade do ar em %
	Luminosidade     int     `json:"luminosidade"`       // Luminosidade 0-100
	UmidadeSolo      int     `json:"umidade_solo"`       // Umidade do solo calibrada em %
	UmidadeSoloBruto int     `json:"umidade_solo_bruto"` // Valor bruto do sensor ADC
	StatusBomba      string  `json:"status_bomba"`       // "Bomba ativada" ou "Bomba desativada"
	StatusLuz        string  `json:"status_luz"`         // "Luz ligada" ou "Luz desligada"
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

	// Registra o novo cliente
	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	log.Printf("✅ Cliente conectado! Total: %d", len(clients))

	// Se existe última leitura, envia imediatamente
	if lastReading != nil {
		jsonData, _ := json.Marshal(lastReading)
		conn.WriteMessage(websocket.TextMessage, jsonData)
		log.Println("📤 Última leitura enviada ao novo cliente")
	}

	// Mantém conexão aberta e aguarda desconexão
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Println("⚠️ Cliente desconectado:", err)
			break
		}
	}

	// Remove cliente quando desconectar
	clientsMu.Lock()
	delete(clients, conn)
	clientsMu.Unlock()

	log.Printf("🔌 Cliente removido. Total: %d", len(clients))
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

	var data SensorData

	// Decodifica JSON recebido da estufa
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Println("❌ Erro ao decodificar JSON:", err)
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Armazena como última leitura
	lastReading = &data

	log.Printf("🌱 Recebido da estufa: Temp=%.1f°C, Umidade=%.1f%%, Luz=%d, Solo=%d%%, StatusBomba=%s, StatusLuz=%s",
		data.Temperatura, data.UmidadeAr, data.Luminosidade, data.UmidadeSolo, data.StatusBomba, data.StatusLuz)

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

// Função para gerar dados simulados (APENAS PARA TESTE - pode comentar/remover)
func generateMockData() SensorData {
	now := time.Now()
	dataHora := now.Format("02/01/2006 15:04:05")

	hora := now.Hour()
	isNoite := hora < 6 || hora > 18

	tempBase := 19.0
	if isNoite {
		tempBase = 16.5
	} else {
		tempBase = 21.0
	}

	umidadeBase := 75.0
	if isNoite {
		umidadeBase = 83.0
	} else {
		umidadeBase = 65.0
	}

	luminosidade := 0
	if !isNoite {
		luminosidade = 75 + (hora%5)*3
	}

	return SensorData{
		DataHora:         dataHora,
		Temperatura:      tempBase + (float64(now.Second()%10) / 10.0),
		UmidadeAr:        umidadeBase + (float64(now.Second()%5) / 5.0),
		Luminosidade:     luminosidade,
		UmidadeSolo:      37 + (now.Second() % 5),
		UmidadeSoloBruto: 1670 + (now.Second() % 20),
		StatusBomba:      "Bomba desativada",
		StatusLuz:        "Luz desligada",
	}
}
