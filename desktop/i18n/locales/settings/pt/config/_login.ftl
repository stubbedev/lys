settings-login-background-mode = Fundo
    .description = Como o fundo do ecrã de início de sessão é desenhado: cor sólida, uma imagem ou a imagem de fundo

settings-login-background-image = Imagem de fundo
    .description = Ficheiro de imagem mostrado atrás do ecrã de início de sessão quando o fundo está definido como "image"

settings-login-background-color = Cor de fundo
    .description = Cor de preenchimento mostrada atrás do ecrã de início de sessão quando o fundo está definido como "color"

settings-login-blur = Desfoque
    .description = Raio do desfoque gaussiano aplicado a fundos de imagem/imagem de fundo (0 = nenhum)

settings-login-show-clock = Mostrar relógio
    .description = Mostrar um relógio acima do pedido de credenciais

settings-login-clock-format = Formato do relógio
    .description = Formato strftime da hora no ecrã de início de sessão (ex.: %H:%M)

settings-login-date-format = Formato da data
    .description = Formato strftime da data no ecrã de início de sessão (ex.: %A, %B %-d)

settings-login-max-attempts = Máximo de tentativas
    .description = Tentativas falhadas de palavra-passe no ecrã de bloqueio antes de a introdução ser bloqueada (0 = ilimitado). O ecrã permanece bloqueado.

settings-login-show-failed-attempts = Mostrar tentativas falhadas
    .description = Mostrar o número de tentativas falhadas no ecrã de bloqueio

settings-login-lock-enabled = Ativado
    .description = Permitir que o lys-shell bloqueie a sessão (responde a loginctl lock-session e lys-shell lock)

settings-login-lock-lock-on-start = Bloquear ao iniciar
    .description = Bloquear a sessão logo que a shell inicia, para configurações de início de sessão automático em que o ecrã de bloqueio é a barreira de acesso

settings-login-lock-grace-period-ms = Período de tolerância
    .description = Intervalo após o bloqueio durante o qual o ecrã se desbloqueia sem palavra-passe (0 = exigi-la sempre)

settings-login-lock-blank-timeout-ms = Tempo até escurecer
    .description = Escurecer o ecrã de bloqueio após este tempo de inatividade (0 = nunca; apenas visual — o DPMS é tarefa do serviço de inatividade)

settings-login-lock-pam-service = Serviço PAM
    .description = Serviço PAM usado para verificar a palavra-passe (ex.: system-auth, login)

settings-login-lock-show-media = Mostrar o que está a tocar
    .description = Mostrar a faixa em reprodução com reproduzir/pausa e seguinte no ecrã de bloqueio (apenas capas locais)

settings-login-lock-show-battery = Mostrar bateria
    .description = Mostrar a carga da bateria no ecrã de bloqueio; ausente sem bateria

settings-login-lock-fingerprint = Impressão digital
    .description = Desbloquear com uma impressão digital registada (fprintd) além da palavra-passe

settings-login-lock-show-switch-user = Mostrar Mudar de utilizador
    .description = Oferecer "Mudar de utilizador" no ecrã de bloqueio quando a máquina tem mais de uma conta

settings-login-greeter-show-user-list = Mostrar lista de utilizadores
    .description = Mostrar avatares clicáveis dos utilizadores da máquina acima do campo do nome de utilizador

settings-login-greeter-show-power-buttons = Mostrar botões de energia
    .description = Mostrar os botões de encerrar/reiniciar no fundo do ecrã de início de sessão

settings-login-greeter-fingerprint = Início de sessão por impressão digital
    .description = Iniciar sessão com uma impressão digital registada (fprintd) além da palavra-passe

settings-login-greeter-match-cursor = Usar o meu cursor
    .description = Mostrar o cursor da última sessão iniciada em vez do tema e do tamanho definidos aqui

settings-login-greeter-cursor-theme = Tema do cursor
    .description = Tema Xcursor usado no ecrã de início de sessão (vazio = predefinição do sistema)

settings-login-greeter-cursor-size = Tamanho do cursor
    .description = Tamanho lógico do cursor; escalado por ecrã para que as saídas HiDPI tenham um cursor nítido

settings-login-greeter-screen-reader = Leitor de ecrã
    .description = Leitor de ecrã que o menu de acessibilidade do ecrã de início de sessão e Super+Alt+S iniciam; oculto quando não está instalado

enum-lock-background-color = Cor
enum-lock-background-image = Imagem
enum-lock-background-wallpaper = Imagem de fundo
