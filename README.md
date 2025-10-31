# 🌱 Mini Estufa API

API em Go para receber dados da mini estufa e distribuir em tempo real via WebSocket.

## 🚀 Desenvolvimento Local

```bash
go run main.go
```

Servidor disponível em `http://localhost:8080`

## 📡 Endpoints

**Estufa envia dados:**
```
POST /api/sensor/push
```

**Dashboard conecta:**
```
WebSocket: ws://localhost:8080/ws
GET: /api/sensor/latest
GET: /health
```

## 🧪 Testar

```bash
# Health check
curl http://localhost:8080/health

# Enviar dados (simular estufa)
curl -X POST http://localhost:8080/api/sensor/push \
  -H "Content-Type: application/json" \
  -d '{
    "data_hora": "31/10/2025 16:30:00",
    "temperatura": 22.3,
    "umidade_ar": 68.5,
    "luminosidade": 78,
    "umidade_solo": 38,
    "umidade_solo_bruto": 1680,
    "status_bomba": "Bomba desativada"
  }'
```

## 🌐 Deploy no Render

1. Push para GitHub
2. Novo Web Service no Render
3. Configure:
   - **Root Directory:** `backend`
   - **Build:** `go build -o server main.go`
   - **Start:** `./server`

A variável `PORT` é configurada automaticamente pelo Render.

## 📊 Formato dos Dados

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

## 🔄 Fluxo

```
Estufa → POST /api/sensor/push → API → WebSocket → Dashboard(s)
```

## 🛠️ Stack

- Go 1.19+
- Gorilla WebSocket
- net/http

---

**Status:** ✅ Pronto para produção

