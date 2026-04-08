# Guia de Uso do Weaver

## Visão Geral

O Weaver ajuda a gerenciar stacks de branches localmente. Você declara as dependências uma vez e usa essas declarações para inspeção, rebase, composição e exportação/importação do estado.

## Configuração

Compile o binário:

```bash
make build
```

Inicialize o Weaver dentro do repositório Git:

```bash
./bin/weaver init
```

Isso cria:

- `.weaver.yaml`
- `.git/weaver/`

## Declarar um Stack

Declare que `feature-b` depende de `feature-a`:

```bash
./bin/weaver stack feature-b --on feature-a
```

Adicione outra branch no topo:

```bash
./bin/weaver stack feature-c --on feature-b
```

Mude a branch para outro pai:

```bash
./bin/weaver stack feature-c --on main
```

Remova a declaração de dependência:

```bash
./bin/weaver unstack feature-c
```

## Inspecionar Dependências

Mostre a árvore completa:

```bash
./bin/weaver deps
```

Mostre a cadeia de uma branch:

```bash
./bin/weaver deps feature-c
```

Saída típica:

```text
main -> feature-a -> feature-b -> feature-c
```

## Verificar a Saúde do Stack

Mostre a árvore com o estado de cada branch:

```bash
./bin/weaver status
```

Estados possíveis:

- `clean`
- `needs rebase`
- `conflict risk`

## Diagnosticar o Estado Local

Rode uma verificação somente leitura:

```bash
./bin/weaver doctor
```

Saída legível por máquina:

```bash
./bin/weaver doctor --json
```

`weaver doctor` verifica inicialização, config, branches declaradas, estado pendente de rebase e problemas comuns de Git, como árvore suja ou operações em andamento.

## Atualizar Branches Locais a Partir do Upstream

Atualize branches explícitas:

```bash
./bin/weaver update main feature-a feature-b
```

Atualize todas as branches rastreadas:

```bash
./bin/weaver update --all
```

Atualize um grupo nomeado:

```bash
./bin/weaver update --group sprint-42
```

`weaver update` roda `git fetch --all` uma vez e depois faz fast-forward de cada branch local selecionada até o upstream configurado. O comando para se uma branch não tiver upstream ou não puder receber fast-forward.

## Rebase de um Stack

Faça rebase de todo o stack até `feature-c`:

```bash
./bin/weaver sync feature-c
```

Se você já estiver na branch alvo:

```bash
./bin/weaver sync
```

Se houver conflito:

```bash
./bin/weaver continue
./bin/weaver abort
```

`continue` retoma depois da resolução manual. `abort` cancela a operação e volta para a branch original.

## Compor Branches

Faça um dry-run:

```bash
./bin/weaver compose feature-c --dry-run
```

Componha várias branches:

```bash
./bin/weaver compose feature-a feature-c feature-e
```

Componha todas as branches rastreadas:

```bash
./bin/weaver compose --all
```

A composição é efêmera por padrão. O comando usa `HEAD` destacado, faz os merges e retorna para a branch original.

Se você quiser criar uma nova branch de integração a partir do resultado composto, faça opt-in explícito:

```bash
./bin/weaver compose feature-b feature-d --base main --create integration
```

Se você quiser recriar uma branch de integração existente a partir da base limpa, faça opt-in explícito:

```bash
./bin/weaver compose feature-b feature-d --base main --update integration
```

Com `--create`, o Weaver cria `integration` a partir do commit composto e depois volta para a branch original.

Com `--update`, o Weaver parte de `main`, recompõe as branches pedidas, move `integration` à força para esse resultado novo e depois volta para a branch original.

## Gerenciar Grupos

Crie um grupo:

```bash
./bin/weaver group create sprint-42 feature-a feature-b feature-c
```

Adicione branches:

```bash
./bin/weaver group add sprint-42 feature-d feature-e
```

Remova branches de um grupo:

```bash
./bin/weaver group remove sprint-42 feature-c
```

Apague o grupo inteiro:

```bash
./bin/weaver group remove sprint-42
```

Liste os grupos:

```bash
./bin/weaver group list
```

Componha um grupo:

```bash
./bin/weaver compose --group sprint-42
```

## Exportar e Importar

Exporte o estado local:

```bash
./bin/weaver export > weaver-state.json
```

Importe em outro clone:

```bash
./bin/weaver import weaver-state.json
```

## Smoke Test

Execute o script de verificação ponta a ponta:

```bash
./smoketest.sh
```

Ele grava um log passo a passo em `smoketest.log`.
