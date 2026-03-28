# 🁣 Dominó Placar

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Deploy](https://img.shields.io/badge/Cloud%20Run-deployed-4285F4?logo=googlecloud&logoColor=white)](#deploy-no-google-cloud-cloud-run)

> Placar digital em tempo real para **Pontinho** — o dominó brasileiro de 51 pontos.
> Rodando no celular de qualquer jogador, sem instalação.
>
> Feito com 🤍 em Diadema, SP — por [leandrodaf](https://github.com/leandrodaf)

---

## Funcionalidades

- **Tempo real** — placar atualiza instantaneamente para todos via Server-Sent Events (SSE)
- **QR Code** — anfitrião compartilha QR code para jogadores entrarem na sala
- **Detecção de pedras por foto** — fotografe as pedras restantes e o sistema reconhece automaticamente (via Roboflow)
- **Torneios multimesa** — suporte a torneios com alocação automática de mesas
- **Mobile-first** — interface pensada para celular, tema escuro premium
- **Zero instalação** — funciona direto no navegador, sem app
- **Hall da Fama** — ranking global persistente entre partidas
- **Apelidos** — sistema de apelidos e votação entre jogadores
- **Dual storage** — SQLite para dev local, Firebase Realtime Database para produção

---

## O que é

**Dominó Placar** é uma aplicação web pensada para ser aberta no celular durante uma partida de dominó. O anfitrião cria a sala, compartilha o QR code, e cada jogador entra com o próprio nome. Quando a rodada acaba, cada um fotografa suas pedras (ou digita os pontos manualmente) e o placar atualiza em tempo real para todo mundo.

---

## Regras do Pontinho

### Objetivo
Ser o último jogador sem estourar. Quem acumular **mais de 51 pontos no total** está fora.

### Pontuação por rodada
- Cada jogador soma os pontos das pedras que restaram na mão ao fim da rodada
- Quem **ganhou a rodada** (jogou todas as pedras, ou travou com menos pontos) marca **0 pontos**
- Demais pedras valem a **soma dos dois lados** de cada peça
- Exceção: a peça `[0|0]` sozinha na mão vale **12 pontos**

### Distribuição de pedras

| Jogadores | Pedras por pessoa | Para comprar | Jogo    |
|:---------:|:-----------------:|:------------:|:-------:|
| 2         | 9                 | 10           | Duplo-6 |
| 3         | 6                 | 10           | Duplo-6 |
| 4         | 5                 | 8            | Duplo-6 |
| 5         | 7                 | 20           | Duplo-9 |
| 6         | 6                 | 19           | Duplo-9 |
| 7         | 5                 | 20           | Duplo-9 |
| 8         | 4                 | 23           | Duplo-9 |
| 9         | 4                 | 19           | Duplo-9 |
| 10        | 3                 | 25           | Duplo-9 |

> **Duplo-6** = 28 pedras · **Duplo-9** = 55 pedras

---

## Rodando localmente

### Pré-requisitos

- **Go 1.26+** — [download](https://go.dev/dl/)

### Instalação

```bash
git clone https://github.com/leandrodaf/domino-placar
cd domino-placar
go mod download
```

### Executar (modo mais simples)

```bash
go run main.go
```

Acesse [http://localhost:8080](http://localhost:8080).

> Por padrão usa **SQLite** local (`domino.db`). Nenhuma variável de ambiente é obrigatória para rodar localmente.

### Jogar na rede Wi-Fi (outros celulares na mesma rede)

Descubra o IP local da sua máquina e acesse pelo celular:

```bash
# macOS
ipconfig getifaddr en0

# Linux
hostname -I | awk '{print $1}'
```

Depois acesse `http://<SEU-IP>:8080` no navegador do celular. Os links de convite e QR codes usam automaticamente o endereço pelo qual você acessou.

---

## Variáveis de ambiente

### Obrigatórias em produção

| Variável | Descrição | Exemplo |
|----------|-----------|---------|
| `SESSION_SECRET` | Segredo HMAC para assinar cookies e tokens CSRF. **Defina sempre em produção.** Sem isso, sessões são invalidadas a cada reinício do servidor. | `minha-chave-secreta-longa` |

### Opcionais

| Variável | Descrição | Padrão |
|----------|-----------|--------|
| `PORT` | Porta HTTP do servidor | `8080` |
| `TRUST_PROXY` | Se definida (qualquer valor), confia no header `X-Forwarded-For` para obter o IP real do cliente (usar apenas atrás de proxy reverso confiável) | não definida |

### Firebase Realtime Database (substituição do SQLite)

| Variável | Descrição |
|----------|-----------|
| `FIREBASE_DATABASE_URL` | URL do banco Firebase, ex: `https://meu-projeto-default-rtdb.firebaseio.com` |
| `FIREBASE_CREDENTIALS` | JSON da Service Account (conteúdo, não caminho). Se omitido, usa Application Default Credentials (ADC) — funciona automaticamente no GCP. |

> Quando `FIREBASE_DATABASE_URL` está definido, o app usa Firebase. Caso contrário, usa SQLite.

### Google Cloud Storage (fotos dos jogadores)

| Variável | Descrição |
|----------|-----------|
| `GCS_BUCKET` | Nome do bucket GCS onde as fotos serão armazenadas, ex: `domino-placar-fotos` |
| `GCS_CREDENTIALS` | JSON da Service Account (conteúdo, não caminho). Se omitido, usa ADC — funciona automaticamente no GCP. |

> Quando `GCS_BUCKET` está definido, as fotos vão para o GCS. Caso contrário, são salvas na pasta `uploads/` local.
### Visão computacional — Roboflow (detecção automática de pedras)

| Variável | Descrição | Padrão |
|----------|-----------|--------|
| `ROBOFLOW_API_KEY` | Chave de API do Roboflow. Sem ela, apenas entrada manual de pontos funciona. | — |
| `ROBOFLOW_MODEL` | Nome do modelo de detecção de dominó no Roboflow | `domino-detection` |
| `ROBOFLOW_VERSION` | Versão do modelo | `1` |

> Quando `ROBOFLOW_API_KEY` está definida, os jogadores podem fotografar as pedras restantes e o sistema detecta automaticamente as peças e calcula os pontos. Sem a chave, o app funciona normalmente com entrada manual.
---

## Deploy no Google Cloud (Cloud Run / Cloud Functions)

### 1. Configure o Firebase

1. Acesse [console.firebase.google.com](https://console.firebase.google.com)
2. Crie um projeto ou use um existente
3. Ative o **Realtime Database** (plano Spark é gratuito)
4. Anote a URL: `https://SEU-PROJETO-default-rtdb.firebaseio.com`

### 2. Configure o GCS (para fotos)

```bash
# Crie o bucket
gsutil mb -l southamerica-east1 gs://domino-placar-fotos

# Permissão pública de leitura (opcional, para exibir fotos na app)
gsutil iam ch allUsers:objectViewer gs://domino-placar-fotos
```

### 3. Build e deploy no Cloud Run

```bash
# Build da imagem
docker build -t gcr.io/SEU-PROJETO/domino-placar .

# Push
docker push gcr.io/SEU-PROJETO/domino-placar

# Deploy
gcloud run deploy domino-placar \
  --image gcr.io/SEU-PROJETO/domino-placar \
  --platform managed \
  --region southamerica-east1 \
  --allow-unauthenticated \
  --set-env-vars "SESSION_SECRET=sua-chave-secreta,FIREBASE_DATABASE_URL=https://SEU-PROJETO-default-rtdb.firebaseio.com,GCS_BUCKET=domino-placar-fotos,TRUST_PROXY=true"
```

> No Cloud Run, as credenciais GCP são automáticas via ADC — não precisa de `FIREBASE_CREDENTIALS` ou `GCS_CREDENTIALS`.

### Dockerfile de exemplo

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o domino-placar .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/domino-placar .
COPY static/ static/
COPY templates/ templates/
EXPOSE 8080
CMD ["./domino-placar"]
```

---

## Rodando com todas as integrações (exemplo .env local)

```bash
# .env (use com: export $(cat .env | xargs) && go run main.go)

SESSION_SECRET=troque-por-uma-string-longa-e-aleatoria

# Firebase (banco de dados)
FIREBASE_DATABASE_URL=https://meu-projeto-default-rtdb.firebaseio.com
# FIREBASE_CREDENTIALS={"type":"service_account",...}  # Só se não estiver no GCP

# GCS (fotos)
GCS_BUCKET=domino-placar-fotos
# GCS_CREDENTIALS={"type":"service_account",...}  # Só se não estiver no GCP

# Roboflow (detecção automática de pedras — opcional)
ROBOFLOW_API_KEY=sua-chave-roboflow
# ROBOFLOW_MODEL=domino-detection
# ROBOFLOW_VERSION=1

PORT=8080
```

Carregue e rode:

```bash
export $(grep -v '^#' .env | xargs) && go run main.go
```

---

## Por que SESSION_SECRET é importante

Sem `SESSION_SECRET`, o servidor gera um segredo aleatório a cada início. Isso significa:

- Cookies de anfitrião e jogador são invalidados a cada reinício
- Tokens CSRF tornam-se inválidos — formulários retornam **"token de segurança inválido"**
- Em desenvolvimento, isso acontece toda vez que você reinicia com `go run`

**Solução**: defina `SESSION_SECRET` com qualquer string longa:

```bash
SESSION_SECRET=qualquer-string-longa-e-dificil-de-adivinhar go run main.go
```

---

## Estrutura do projeto

```
domino-placar/
├── main.go                      # Ponto de entrada, roteamento, middleware
├── go.mod
├── domino.db                    # Banco SQLite (criado automaticamente, dev only)
├── uploads/                     # Fotos locais (dev only; em prod vai para GCS)
├── static/
│   └── style.css                # Design system — tema escuro mobile-first
├── templates/
│   ├── base.html                # Layout base com nav e footer
│   ├── home.html                # Página inicial
│   ├── lobby.html               # Sala de espera (anfitrião)
│   ├── join.html                # Entrar na partida (jogador)
│   ├── waiting.html             # Aguardando início
│   ├── game.html                # Mesa de jogo em tempo real
│   ├── upload.html              # Fotografar pedras / entrada manual
│   ├── confirm.html             # Confirmar pontuação da rodada
│   ├── ranking.html             # Placar da partida
│   ├── nicknames.html           # Apelidos e votação
│   ├── global-ranking.html      # Hall da Fama global
│   └── tournament-*.html        # Torneios multimesa
└── internal/
    ├── db/
    │   ├── store.go             # Interface Store (abstração de banco)
    │   ├── sqlite_store.go      # Implementação SQLite
    │   ├── firebase_store.go    # Implementação Firebase Realtime DB
    │   └── db.go                # Schema e helpers SQLite
    ├── models/models.go         # Structs de domínio
    ├── handler/
    │   ├── security.go          # HMAC cookies, CSRF, rate limiting, headers
    │   ├── pages.go             # Handlers de renderização de páginas
    │   ├── match.go             # Criar/iniciar partida
    │   ├── round.go             # Gestão de rodadas e vencedores
    │   ├── upload.go            # Upload de fotos e confirmação
    │   ├── nickname.go          # Sistema de apelidos
    │   ├── tournament.go        # Torneios multimesa
    │   ├── tiles.go             # Cálculo de distribuição de pedras
    │   └── sse.go               # Server-Sent Events (tempo real)
    └── service/
        ├── image.go             # Compressão e validação de imagens
        ├── storage.go           # Upload para Google Cloud Storage
        ├── vision.go            # Detecção de pedras via Roboflow (visão computacional)
        └── qrcode.go            # Geração de QR codes
```

---

## Segurança

- **Cookies HMAC-signed**: anfitrião e jogadores autenticados por cookie criptografado
- **CSRF tokens**: baseados em HMAC, rotação por hora, validados em todos os formulários POST
- **Rate limiting**: 5 uploads/5min por IP · 60 ações POST/min por IP
- **Sanitização de entrada**: todos os campos de usuário são sanitizados no servidor
- **CSP**: Content-Security-Policy restritivo em todas as respostas
- **Score máximo**: pontuações acima de 200 são rejeitadas pelo servidor

---

## Contribuindo

Contribuições são bem-vindas! Sinta-se à vontade para abrir issues e pull requests.

1. Fork o projeto
2. Crie uma branch (`git checkout -b feat/minha-feature`)
3. Commit suas mudanças (`git commit -m 'feat: minha feature'`)
4. Push para a branch (`git push origin feat/minha-feature`)
5. Abra um Pull Request

---

## Licença

Este projeto está sob a licença MIT — veja o arquivo [LICENSE](LICENSE) para detalhes.

Feito com 🤍 em Diadema, SP.
