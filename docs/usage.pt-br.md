# Guia de Uso do Weaver

## Visão Geral

O Weaver ajuda a gerenciar stacks de branches localmente. Você declara as dependências uma vez e usa essas declarações para inspeção, rebase, composição e exportação/importação do estado.

## Configuração

Instale a release estável mais recente:

```bash
go install github.com/lutefd/weaver@latest
weaver version
```

Inicialize o Weaver dentro do repositório Git:

```bash
weaver init
```

Isso cria:

- `.weaver.yaml`
- `.git/weaver/`

## Escolha o Comando Certo

- Use `weaver stack` e `weaver unstack` para declarar ou mudar o grafo de dependências entre branches.
- Use `weaver deps`, `weaver status` e `weaver doctor` quando quiser visibilidade somente leitura do estado atual do stack.
- Use `weaver update` quando quiser atualizar branches locais a partir dos seus próprios upstreams configurados.
- Use `weaver sync` quando quiser colocar o stack em ordem de dependência de novo, aplicando cada parent na sua child.
- Use `weaver compose` quando quiser prever ou materializar o resultado combinado de várias branches em cima de uma base sem alterar as branches de origem.
- Use `weaver integration ...` quando a receita de composição precisar ser salva, nomeada, reutilizada e compartilhada entre clones.

A diferença importante é:

- `weaver update` segue o upstream de cada branch, como `origin/feature-a`.
- `weaver sync` segue o parent declarado no stack, como `feature-a` sobre `main` e `feature-b` sobre `feature-a`.
- `weaver compose` não atualiza as branches do stack em si. Ele testa ou monta o resultado combinado delas sobre uma base escolhida.

## Declarar um Stack

Declare que `feature-b` depende de `feature-a`:

```bash
weaver stack feature-b --on feature-a
```

Adicione outra branch no topo:

```bash
weaver stack feature-c --on feature-b
```

Mude a branch para outro pai:

```bash
weaver stack feature-c --on main
```

Remova a declaração de dependência:

```bash
weaver unstack feature-c
```

## Inspecionar Dependências

Mostre a árvore completa:

```bash
weaver deps
```

Mostre a cadeia de uma branch:

```bash
weaver deps feature-c
```

Saída típica:

```text
main -> feature-a -> feature-b -> feature-c
```

## Verificar a Saúde do Stack

Mostre a árvore com o estado de cada branch:

```bash
weaver status
```

Estados possíveis:

- `clean`
- `needs rebase`
- `conflict risk`

## Diagnosticar o Estado Local

Rode uma verificação somente leitura:

```bash
weaver doctor
```

Saída legível por máquina:

```bash
weaver doctor --json
```

`weaver doctor` verifica inicialização, config, branches declaradas, estado pendente de rebase e problemas comuns de Git, como árvore suja ou operações em andamento.

## Atualizar Branches Locais a Partir do Upstream

Atualize branches explícitas:

```bash
weaver update main feature-a feature-b
```

Atualize todas as branches rastreadas:

```bash
weaver update --all
```

Atualize um grupo nomeado:

```bash
weaver update --group sprint-42
```

Atualize todas as branches rastreadas por uma estratégia de integração salva:

```bash
weaver update --integration integration
```

`weaver update` roda `git fetch --all` uma vez e depois faz fast-forward de cada branch local selecionada até o upstream configurado. O comando para se uma branch não tiver upstream ou não puder receber fast-forward.

## Rebase de um Stack

Use o modo padrão com rebase quando:

- você quiser histórico linear e limpo para stacked diffs
- você aceitar reescrever o histórico das branches e dar force-push depois
- o stack estiver sob controle de uma pessoa ou de um time que já trabalha num fluxo rebase-first

Use merge em vez disso quando:

- as branches já tiverem PRs abertos ou comentários ativos de review
- outras pessoas já estiverem consumindo exatamente aquelas pontas de branch
- preservar o histórico das branches importar mais do que mantê-lo linear

No Weaver, rebase deixa o stack mais limpo, enquanto merge deixa o histórico das branches mais estável.

Faça rebase de todo o stack até `feature-c`:

```bash
weaver sync feature-c
```

Se você já estiver na branch alvo:

```bash
weaver sync
```

Se as branches já tiverem PRs abertos e você quiser preservar o histórico, faça merge de cada parent no stack. Quando possível o Git faz fast-forward; caso contrário, grava o merge commit normal:

```bash
weaver sync feature-c --merge
```

Se houver conflito:

```bash
weaver continue
weaver abort
```

`continue` retoma depois da resolução manual. `abort` cancela a operação e volta para a branch original. Os mesmos comandos funcionam tanto para sync com rebase quanto para sync com merge.

`weaver update` não substitui `weaver sync` nem `weaver sync --merge`. `update` só atualiza as branches a partir dos upstreams configurados. Ele não traz `main` para `feature-a` nem `feature-a` para `feature-b`.

## Compor Branches

Faça um dry-run:

```bash
weaver compose feature-c --dry-run
```

Componha várias branches:

```bash
weaver compose feature-a feature-c feature-e
```

Componha todas as branches rastreadas:

```bash
weaver compose --all
```

Pule uma branch problemática e mantenha o restante da composição:

```bash
weaver compose --integration integration --create integration-preview --skip feature-debug-search-api-curl
```

A composição é efêmera por padrão. O comando usa `HEAD` destacado, faz os merges e retorna para a branch original.

Use `weaver compose` quando precisar responder perguntas como:

- "Essas branches entram limpas juntas em cima de `main`?"
- "Posso criar uma branch de integração para QA ou staging?"
- "Qual branch é a outlier barulhenta que vale pular e mergear manualmente depois?"

Prefira `weaver sync` quando o objetivo for atualizar as branches reais do stack. Prefira `weaver compose` quando o objetivo for inspecionar ou montar o resultado combinado.

Se você quiser criar uma nova branch de integração a partir do resultado composto, faça opt-in explícito:

```bash
weaver compose feature-b feature-d --base main --create integration
```

Se você quiser recriar uma branch de integração existente a partir da base limpa, faça opt-in explícito:

```bash
weaver compose feature-b feature-d --base main --update integration
```

Com `--create`, o Weaver cria `integration` a partir do commit composto e depois volta para a branch original.

Com `--update`, o Weaver parte de `main`, recompõe as branches pedidas, move `integration` à força para esse resultado novo e depois volta para a branch original.

Se a composição encontrar conflito, o Weaver informa qual branch falhou e quais arquivos entraram em conflito.

Se você não tiver passado `--skip`, o Weaver pergunta se quer pular a branch com problema ou abortar a composição.

Se uma branch estiver muito divergente e continuar quebrando uma composição grande, normalmente é melhor remover essa branch da composição ou da integração salva, corrigi-la primeiro, e depois mergeá-la manualmente na branch produzida por `--create` ou `--update` antes de colocá-la de volta quando estiver estável.

Se você já tiver salvo uma estratégia reutilizável, pode compor direto dela:

```bash
weaver compose --integration integration --update integration
```

Ao usar `--integration`, o Weaver pega a base e a lista de branches da estratégia salva.

## Gerenciar Integrações Salvas

Salve ou atualize uma estratégia:

```bash
weaver integration save integration --base main feature-a feature-b feature-c
```

Mostre a estratégia:

```bash
weaver integration show integration
```

Diagnostique a estratégia:

```bash
weaver integration doctor integration
weaver integration doctor integration --json
```

Liste as estratégias salvas:

```bash
weaver integration list
```

Remova uma estratégia:

```bash
weaver integration remove integration
```

Exporte uma estratégia em JSON:

```bash
weaver integration export integration --json > integration.json
```

Importe em outro clone:

```bash
weaver integration import integration.json
```

## Gerenciar Grupos

Crie um grupo:

```bash
weaver group create sprint-42 feature-a feature-b feature-c
```

Adicione branches:

```bash
weaver group add sprint-42 feature-d feature-e
```

Remova branches de um grupo:

```bash
weaver group remove sprint-42 feature-c
```

Apague o grupo inteiro:

```bash
weaver group remove sprint-42
```

Liste os grupos:

```bash
weaver group list
```

Componha um grupo:

```bash
weaver compose --group sprint-42
```

## Exportar e Importar

Exporte o estado local, incluindo integrações salvas:

```bash
weaver export > weaver-state.json
```

Importe em outro clone:

```bash
weaver import weaver-state.json
```

## Smoke Test

Execute o script de verificação ponta a ponta:

```bash
./smoketest.sh
```

Ele grava um log passo a passo em `smoketest.log`.
