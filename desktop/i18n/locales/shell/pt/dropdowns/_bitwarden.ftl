dropdown-bitwarden-title = Bitwarden
dropdown-bitwarden-tooltip-logged-out = Bitwarden: inicie sessão para abrir o cofre
dropdown-bitwarden-tooltip-locked = Bitwarden: o cofre está bloqueado
dropdown-bitwarden-tooltip-unlocked = Bitwarden: o cofre está aberto
dropdown-bitwarden-unavailable = Cofre indisponível
dropdown-bitwarden-server = Servidor: { $server }
dropdown-bitwarden-account = Bloqueado: { $email }
dropdown-bitwarden-email = Email
dropdown-bitwarden-email-placeholder = voce@exemplo.com
dropdown-bitwarden-master-password = Palavra-passe mestra
dropdown-bitwarden-client-id = client_id da chave de API
dropdown-bitwarden-client-secret = client_secret da chave de API
dropdown-bitwarden-required = Obrigatório
dropdown-bitwarden-login = Iniciar sessão
dropdown-bitwarden-login-tooltip = Iniciar sessão na sua conta Bitwarden
dropdown-bitwarden-use-api-key = Iniciar sessão com uma chave de API
dropdown-bitwarden-use-api-key-tooltip = Iniciar sessão com a sua chave de API pessoal, que não precisa de segundo fator
dropdown-bitwarden-use-password = Iniciar sessão com a palavra-passe mestra
dropdown-bitwarden-use-password-tooltip = Iniciar sessão com o seu email e palavra-passe mestra
dropdown-bitwarden-two-factor-title = Início de sessão em dois passos
dropdown-bitwarden-two-factor-app = Introduza o código da sua aplicação de autenticação.
dropdown-bitwarden-two-factor-email = Introduza o código enviado por email para { $email }.
dropdown-bitwarden-new-device-title = Novo dispositivo
dropdown-bitwarden-new-device-desc = O Bitwarden enviou um código de verificação para { $email } para confirmar este dispositivo.
dropdown-bitwarden-code-placeholder = Código
dropdown-bitwarden-send-code = Enviar código
dropdown-bitwarden-send-code-tooltip = Enviar o código de verificação para o endereço da sua conta
dropdown-bitwarden-back = Voltar
dropdown-bitwarden-back-login-tooltip = Voltar ao início de sessão
dropdown-bitwarden-back-vault-tooltip = Voltar ao cofre
dropdown-bitwarden-verify = Verificar
dropdown-bitwarden-verify-tooltip = Verificar o código e concluir o início de sessão
dropdown-bitwarden-unlock = Desbloquear
dropdown-bitwarden-unlock-tooltip = Abrir o cofre com a sua palavra-passe mestra
dropdown-bitwarden-sync = Sincronizar
dropdown-bitwarden-lock = Bloquear
dropdown-bitwarden-logout = Terminar sessão
dropdown-bitwarden-search = Pesquisar no cofre
dropdown-bitwarden-no-items = Sem itens
dropdown-bitwarden-more = A mostrar { $shown } de { $total } itens. Pesquise para os filtrar.
dropdown-bitwarden-undecryptable = Não foi possível desencriptar
dropdown-bitwarden-copy = Copiar
dropdown-bitwarden-copy-reprompt-tooltip = Verificar a palavra-passe mestra e copiar o campo protegido
dropdown-bitwarden-cancel = Cancelar
dropdown-bitwarden-cancel-tooltip = Não copiar nada e voltar ao cofre
dropdown-bitwarden-copy-field = Copiar { $field }
dropdown-bitwarden-copied = Copiado: { $field }.
dropdown-bitwarden-copied-clears = Copiado: { $field }. A área de transferência é limpa em { $seconds } s.
dropdown-bitwarden-reprompt-title = Nova solicitação da palavra-passe mestra
dropdown-bitwarden-reprompt-desc = { $item } está protegido: introduza a sua palavra-passe mestra para copiar o campo { $field }.
dropdown-bitwarden-field-username = nome de utilizador
dropdown-bitwarden-field-password = palavra-passe
dropdown-bitwarden-field-totp = código de verificação
dropdown-bitwarden-field-card-number = número do cartão
dropdown-bitwarden-field-card-code = código de segurança
dropdown-bitwarden-field-email = email
dropdown-bitwarden-field-notes = nota
dropdown-bitwarden-field-passphrase = frase-passe
dropdown-bitwarden-generator = Gerar uma palavra-passe
dropdown-bitwarden-generate = Gerar e copiar: { $field }
dropdown-bitwarden-generator-password-caption = { $length } caracteres: { $charsets }
dropdown-bitwarden-generator-passphrase-caption = { $words } palavras
dropdown-bitwarden-generator-empty = Sem predefinições do gerador
dropdown-bitwarden-generator-empty-hint = Adicione-as em modules.bitwarden.generator-passwords e generator-passphrases.
dropdown-bitwarden-error-wrong-password = Palavra-passe mestra errada.
dropdown-bitwarden-error-credentials = O email, a palavra-passe mestra ou o código estão incorretos.
dropdown-bitwarden-error-login-required = A sessão terminou. Inicie sessão novamente.
dropdown-bitwarden-error-unavailable = O servidor está indisponível. Tente novamente mais tarde.
dropdown-bitwarden-error-server = O servidor recusou: { $message }
dropdown-bitwarden-error-network = Não foi possível contactar o servidor.
launcher-bitwarden-locked = O Bitwarden está bloqueado. Escreva a sua palavra-passe mestra e prima Enter.
launcher-bitwarden-logged-out = Inicie primeiro sessão no Bitwarden a partir do seu menu pendente na barra.
launcher-bitwarden-hint = Enter copia a palavra-passe ou o primeiro campo de um item; Shift+Enter lista os seus campos.
dropdown-bitwarden-totp-live = { $code } · faltam { $seconds } s
dropdown-bitwarden-notify-sync-failed = Não foi possível sincronizar o Bitwarden
dropdown-bitwarden-notify-logged-out = O Bitwarden terminou a sua sessão
dropdown-bitwarden-notify-logged-out-body = A sessão expirou ou foi revogada. Inicie sessão novamente a partir da barra.
dropdown-bitwarden-sends = Bitwarden Send
dropdown-bitwarden-send-name = Nome
dropdown-bitwarden-send-name-placeholder = O que está a enviar
dropdown-bitwarden-send-text = Texto
dropdown-bitwarden-send-text-placeholder = O texto que a ligação abre
dropdown-bitwarden-send-deletion = Eliminar após
dropdown-bitwarden-send-max-access = Máximo de aberturas (0 para sem limite)
dropdown-bitwarden-send-password = Palavra-passe
dropdown-bitwarden-send-password-placeholder = Opcional, pedida antes de o texto ser mostrado
dropdown-bitwarden-send-create = Criar e copiar ligação
dropdown-bitwarden-send-create-tooltip = Criar o Send e copiar a sua ligação de partilha
dropdown-bitwarden-send-none = Sem Sends
dropdown-bitwarden-send-delete = Eliminar { $name }
dropdown-bitwarden-send-hours = { $hours ->
    [one] { $hours } hora
    [many] { $hours } horas
   *[other] { $hours } horas
}
dropdown-bitwarden-send-days = { $days ->
    [one] { $days } dia
    [many] { $days } dias
   *[other] { $days } dias
}
dropdown-bitwarden-send-accesses = { $count ->
    [one] aberto { $count } vez
    [many] aberto { $count } vezes
   *[other] aberto { $count } vezes
}
dropdown-bitwarden-send-accesses-of = aberto { $count } de { $max } vezes
dropdown-bitwarden-send-deletes = eliminado a { $date }
dropdown-bitwarden-send-protected = palavra-passe
dropdown-bitwarden-send-disabled = desativado
dropdown-bitwarden-field-send-link = Ligação do Send
