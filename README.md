# 🁣 Dominó Placar

> Placar digital em tempo real para **Pontinho** — o dominó brasileiro de 51 pontos.
> Rodando no celular de qualquer jogador, sem instalação.
>
> Feito com 🤍 em Diadema, SP — por [leandrodaf](https://github.com/leandrodaf)

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

- **Go 1.23+** — [download](https://go.dev/dl/)

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

```bash
HOST=192.168.1.100 go run main.go
```

Substitua `192.168.1.100` pelo IP da sua máquina na rede local. Os QR codes apontarão para esse endereço.

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
| `HOST` | Hostname/IP para QR codes e links de convite | `localhost` |
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
FROM golang:1.23-alpine AS builder
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

HOST=192.168.1.100
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

## Licença

Projeto pessoal — uso livre. Feito com 🤍 em Diadema, SP.
