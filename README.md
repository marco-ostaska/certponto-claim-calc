# Certponto Claim Calc

Calculadora de claims mensais para regime de plantão CertPonto. Suporta dois modos de cálculo (jeito atual e uniforme) e considera feriados.

## Modos

- **Jeito atual (Modo A)**: Distribuição conforme regras vigentes
- **Uniforme (Modo B)**: Distribuição uniforme das horas no mês

## Uso

### Web (Vercel)
Acesse a URL do deploy e preencha o formulário.

### CLI

```bash
# Calcular mês atual
./calc Mar-2026

# Com feriados
./calc Mar-2026 --feriados 21,25

# Modo uniforme
./calc Mar-2026 --modo uniforme
```

### API

```
GET /api/calc?month=Mar-2026&feriados=&modo=a
```

## Deploy

Projeto configurado para deploy no Vercel via `vercel.json`.
