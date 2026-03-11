# hack-fiap233-users

Microsserviço de usuários em Go.

## Endpoints

| Método | Rota | Descrição |
|---|---|---|
| GET | `/users/health` | Health check + status do banco |
| GET | `/users/` | Listar usuários |
| POST | `/users/` | Criar usuário (`{"name":"...","email":"..."}`) |

## Rodar localmente

```bash
go run main.go
```

## Deploy

O deploy é automático via GitHub Actions. Qualquer push na `main` executa:

1. Build da imagem Docker
2. Push para o ECR
3. Deploy no cluster EKS

### Secrets necessárias no GitHub

| Secret | Descrição |
|---|---|
| `AWS_ACCESS_KEY_ID` | Access Key da AWS Academy |
| `AWS_SECRET_ACCESS_KEY` | Secret Key da AWS Academy |
| `AWS_SESSION_TOKEN` | Session Token da AWS Academy |
