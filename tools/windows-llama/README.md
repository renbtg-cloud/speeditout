# Speedit out — llama.cpp no Windows via Git Bash

Este kit **substitui** o kit Linux anterior. Ele roda em Windows 10/11 x64 com Git Bash instalado. **Não requer WSL, Docker, Visual Studio, CMake, GCC ou Python** para instalar e executar o primeiro teste (Python é opcional para mostrar JSON formatado). Usa os binários Windows distribuídos pelo WinGet.

## Passo a passo

1. Extraia o ZIP usando o Explorer do Windows, por exemplo em `C:\Users\SEU_USUARIO\Downloads\speeditout-llama-windows-gitbash`.
2. Abra **Git Bash** na pasta extraída (botão direito: Open Git Bash Here).
3. Execute, um comando de cada vez:

    ```bash
    bash 00-check.sh
    bash 01-install.sh
    ```

4. **Continue no mesmo Git Bash.** Não precisa reiniciar nem executar `export PATH`: os scripts agora procuram o executável nas pastas do WinGet.
5. Ainda no primeiro terminal:

    ```bash
    bash 02-serve.sh
    ```

    Ele inicia o servidor apenas em `127.0.0.1:18913`. Se você não definir um GGUF local, o primeiro uso baixa `ggml-org/Qwen3-1.7B-GGUF:Q4_K_M` e guarda no cache.
6. Abra um **segundo Git Bash** na mesma pasta:

    ```bash
    bash 03-test.sh
    bash 04-benchmark.sh
    ```

7. Resultados em `~/speeditout-llama-windows/results/`. Copie o `benchmark.csv` e `answer.json` para nossa análise.

## Reutilizar um GGUF já existente

No **primeiro terminal** (onde iniciará o servidor):

```bash
export LLAMA_MODEL_FILE='/c/Users/SEU_USUARIO/Downloads/Qwen3-1.7B-Q4_K_M.gguf'
bash 02-serve.sh
```

Use caminhos Git Bash (`/c/Users/...`) com aspas simples para espaços. O script converte para caminho Windows para o executável nativo. A variável é opcional; se omitida o `llama.cpp` baixa a variante Q4_K_M.

Para comparar rigorosamente o mesmo GGUF de antes, veja se o SHA256 corresponde a `d2387ca2dbfee2ffabce7120d3770dadca0b293052bc2f0e138fdc940d9bc7b5`.

## Ajustes

```bash
export LLAMA_THREADS=4
export LLAMA_CTX=2048
export LLAMA_PORT=18913
bash 02-serve.sh
```

Considere começar com 4 threads CPU; não deduza o melhor número apenas pelo total de núcleos. Mais contextos e concorrência exigem mais memória.

## Se a instalação não for encontrada

Mesmo após WinGet instalar com sucesso, alguns pacotes podem ficar em locais diferentes. Caso `02-serve.sh` não encontre o programa, use `export LLAMA_SERVER_BIN='/c/CAMINHO/llama-server.exe'` no terminal antes de rodá-lo; isso dispensa modificar PATH.

## Se WinGet não existir

Use o [release oficial do llama.cpp](https://github.com/ggml-org/llama.cpp/releases), escolha o ZIP **Windows x64 CPU**, extraia todo seu conteúdo preservando as DLLs, e coloque o diretório dos `.exe` no PATH ou configure o `LLAMA_SERVER_BIN` apontando para `llama-server.exe`:

```bash
export LLAMA_SERVER_BIN='/c/Users/SEU_USUARIO/Downloads/llama-bin/llama-server.exe'
bash 02-serve.sh
```

Caso prefira compilar o fonte com MSVC, a documentação oficial recomenda **Developer Command Prompt/PowerShell do Visual Studio**, com workload C++, não uma simples sessão Git Bash. Não é necessário compilar para nosso teste.

## O que o kit não faz

- Não instala Linux, WSL, Docker ou ferramentas de desenvolvimento C++.
- Não abre a API na rede externa; somente loopback.
- Não envia seu prompt ao servidor remoto; `-hf` baixa os pesos quando necessário e a inferência roda no seu PC.
- Não mede a correção semântica de um benchmark completo; apenas operação e latência básica.
- A primeira execução precisa de internet para instalar o binário e baixar o GGUF.
- Não foi executado no Windows real neste ambiente; scripts foram testados por inspeção de Bash e simulações de chamadas do Git Bash.

Referências: [Instalação oficial](https://github.com/ggml-org/llama.cpp/blob/master/docs/install.md), [Llama serve](https://llama.app/docs/serve), [modelo oficial GGUF](https://huggingface.co/ggml-org/Qwen3-1.7B-GGUF).
