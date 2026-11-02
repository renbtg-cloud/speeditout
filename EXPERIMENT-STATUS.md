# Speedit out v0.5 — status: primeiro Qwen REAL

## Falhas primeiro

- **17/32** verificações de intenção/decisão falharam no **Go + Qwen real**, e **15/32 passaram**.
- **18/32** falharam no **Go sozinho**, e **14/32 passaram**.
- Ganho líquido: **1 correção**, **0 regressões**, **1 decisão alterada**. A correção foi no caso `N06` (rejeição de jogo); o controlador Go continua frequentemente bloqueando rótulos semanticamente plausíveis, e o modelo frequentemente escolhe `unknown`.
- Na primeira amostra de classificações isoladas por logits, **2/4** falharam e **2/4** passaram, com erros em `N02` e `N04`.
- Os testes automatizados de runtime GGUF passaram **6/6 (0 falhas)**; a suíte Go com race detector passou, assim como `go vet`.

## O que foi feito, comprovadamente

1. SHA-256 do GGUF de 1.282.439.264 bytes verificado: `d2387ca2dbfee2ffabce7120d3770dadca0b293052bc2f0e138fdc940d9bc7b5`.
2. Foram interpretados metadados GGUF: arquitetura Qwen3, 28 camadas, 311 tensores.
3. Um runtime experimental próprio desquantizou tensores Q4_K/Q6_K, aplicou Qwen3 forward e gerou efetivamente o token `4` para uma pergunta aritmética simples (29,39s, incluindo preparação de pesos).
4. Foi criada uma inferência otimizada por seleção de sete tokens-letra candidatos, sem gerar resposta extensa; o backend local respondeu **21 requisições de inferência real** durante o teste de integração com Go, média observada **6,09 s/requisição** (5,56–7,21 s). Não extrapolar para concorrência alta.
5. Go manteve sua própria decisão via `Admit`, com fallback determinístico, sem envio automático de mensagens.
6. O serviço experimental foi limitado ao loopback e foram usados apenas casos sintéticos.

## Limitações técnicas importantes

- Não foi possível instalar `llama.cpp` nesta VM por indisponibilidade de DNS/acesso ao GitHub. O runtime Python é **experimental** e não tem equivalência numérica validada contra um motor de referência.
- A ponte HTTP é adaptada a **este experimento**: usa logits de letras para a classificação, constrói o JSON no host e extrai a citação diretamente da mensagem do usuário; o modelo **não** escreveu nem verificou a citação e não gerou um JSON completo.
- A inferência foi executada no CPU; multiplicações de matrizes são feitas por PyTorch e o motor usa arquivos derivados em disco de ~3,8 GiB. Não atende à exigência de produção/Highway Star.
- Embora seja uma melhoria líquida em **um** caso, isso não fornece evidência estatística de superioridade ao modelo sozinho, ao GPT-6 ou a outros fornecedores.
- Nem Niccolò, nem RAG vetorial, nem os ciclos evolutivos do GEQO foram incorporados nesta rodada.

## Próximo passo técnico que realmente importa

Usar um runtime de referência otimizado para GGUF (llama.cpp), conferir se nossos logits e classificações coincidem com os de referência, medir qualidade e latência sem seleção forçada de letras, e desenvolver um lote novo e genuinamente cego para comparação rigorosa com Go-only e modelos maiores. Só então otimizar concorrência/throughput do caminho semântico.
