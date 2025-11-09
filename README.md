# 🌱 Mini Estufa API

API em Go para receber dados da mini estufa e distribuir em tempo real via WebSocket.

## 🚀 Desenvolvimento Local

```bash
go run main.go
```

Servidor disponível em `http://localhost:8080`

## 📡 Endpoints

### Para a Estufa enviar dados:
```
POST /api/sensor/push
Content-Type: application/json
```

### Para o Dashboard:
```
WebSocket: ws://localhost:8080/ws
GET: /api/sensor/latest
GET: /health
```

## 🧪 Testar Localmente

```bash
# Health check
curl http://localhost:8080/health

# Enviar dados (simular estufa)
curl -X POST http://localhost:8080/api/sensor/push \
  -H "Content-Type: application/json" \
  -d '{
    "data_hora": "03/11/2025 16:30:00",
    "temperatura": 22.3,
    "umidade_ar": 68.5,
    "luminosidade": 78,
    "umidade_solo": 38,
    "umidade_solo_bruto": 1680,
    "status_bomba": "Bomba desativada"
  }'

# Buscar última leitura
curl http://localhost:8080/api/sensor/latest
```

## 🌐 Deploy no Render

### Configuração Inicial

1. **Push para GitHub**
   ```bash
   git add .
   git commit -m "Backend WebSocket ready for production"
   git push origin main
   ```

2. **Criar Web Service no Render**
   - Acesse https://render.com
   - New > Web Service
   - Conecte seu repositório
   - Configure:
     - **Name:** miniestufa-backend
     - **Root Directory:** `backend` (se o código Go estiver em subpasta)
     - **Runtime:** Go
     - **Build Command:** `go build -o server main.go`
     - **Start Command:** `./server`

3. **Variáveis de Ambiente**
   - `PORT` é configurado automaticamente pelo Render ✅
   - Não precisa configurar nada manualmente

### URLs de Produção

Após deploy, suas URLs serão:

- **Base URL:** `https://miniestufa-backend.onrender.com`
- **WebSocket:** `wss://miniestufa-backend.onrender.com/ws`
- **API REST:** `https://miniestufa-backend.onrender.com/api/sensor/latest`
- **Push Endpoint:** `https://miniestufa-backend.onrender.com/api/sensor/push`
- **Health Check:** `https://miniestufa-backend.onrender.com/health`

### Testar Produção

```bash
# Health check
curl https://miniestufa-backend.onrender.com/health

# Enviar dados de teste
curl -X POST https://miniestufa-backend.onrender.com/api/sensor/push \
  -H "Content-Type: application/json" \
  -d '{
    "data_hora": "03/11/2025 16:30:00",
    "temperatura": 22.3,
    "umidade_ar": 68.5,
    "luminosidade": 78,
    "umidade_solo": 38,
    "umidade_solo_bruto": 1680,
    "status_bomba": "Bomba desativada"
  }'
```

## 📊 Formato dos Dados

### Entrada (da Estufa)
```json
{
  "data_hora": "DD/MM/YYYY HH:MM:SS",
  "temperatura": 21.5,
  "umidade_ar": 65.3,
  "luminosidade": 85,
  "umidade_solo": 39,
  "umidade_solo_bruto": 1675,
  "status_bomba": "Bomba desativada"
}
```

### Saída (para Dashboard)
O mesmo formato JSON é transmitido via WebSocket para todos os dashboards conectados.

## 🔄 Fluxo de Dados

```
Estufa ESP32
    ↓ POST /api/sensor/push
Backend API (Go)
    ↓ WebSocket broadcast
Dashboard(s) conectados
```

## ⚙️ Funcionalidades

- ✅ **WebSocket em tempo real** - Múltiplos clientes conectados simultaneamente
- ✅ **CORS habilitado** - Aceita conexões de qualquer origem
- ✅ **Reconexão automática** - Clientes reconectam se perderem conexão
- ✅ **Última leitura armazenada** - Novos clientes recebem dados imediatamente
- ✅ **Health check** - Para monitoramento de uptime
- ✅ **Porta dinâmica** - Suporta deploy em Render, Heroku, etc

## 🛠️ Stack Tecnológica

- **Go 1.19+** - Linguagem principal
- **Gorilla WebSocket** - Gerenciamento de conexões WebSocket
- **net/http** - Servidor HTTP nativo
- **encoding/json** - Serialização de dados

## 📝 Dependências

```bash
go get github.com/gorilla/websocket
```

## 🐛 Troubleshooting

### Erro: "websocket: the client is not using the websocket protocol"

**Causa:** Cliente tentando conectar com HTTP ao invés de WebSocket

**Solução:** Certifique-se de usar:
- Desenvolvimento: `ws://localhost:8080/ws`
- Produção: `wss://miniestufa-backend.onrender.com/ws` (com SSL)

### Erro: "connection refused"

**Causa:** Servidor não está rodando

**Solução:**
```bash
# Local
go run main.go

# Produção
Verifique logs no Render e faça restart se necessário
```

### Render dormindo após 15 min (plano free)

**Causa:** Inatividade no plano gratuito

**Solução:**
- Configure um cron job externo para fazer ping
- Ou faça upgrade para plano pago

---

**Status:** ✅ Pronto para produção  
**Deploy atual:** https://miniestufa-backend.onrender.com
