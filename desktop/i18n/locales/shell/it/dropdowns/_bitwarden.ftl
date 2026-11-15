### Bitwarden Dropdown

dropdown-bitwarden-title = Bitwarden
dropdown-bitwarden-tooltip-logged-out = Bitwarden: accedi per aprire la cassaforte
dropdown-bitwarden-tooltip-locked = Bitwarden: la cassaforte è bloccata
dropdown-bitwarden-tooltip-unlocked = Bitwarden: la cassaforte è aperta
dropdown-bitwarden-unavailable = Cassaforte non disponibile
dropdown-bitwarden-server = Server: { $server }
dropdown-bitwarden-account = Bloccata: { $email }
dropdown-bitwarden-email = Email
dropdown-bitwarden-email-placeholder = tu@example.com
dropdown-bitwarden-master-password = Password principale
dropdown-bitwarden-client-id = client_id della chiave API
dropdown-bitwarden-client-secret = client_secret della chiave API
dropdown-bitwarden-required = Obbligatorio
dropdown-bitwarden-login = Accedi
dropdown-bitwarden-login-tooltip = Accedi al tuo account Bitwarden
dropdown-bitwarden-use-api-key = Accedi con una chiave API
dropdown-bitwarden-use-api-key-tooltip = Accedi con la tua chiave API personale, che non richiede un secondo fattore
dropdown-bitwarden-use-password = Accedi con la password principale
dropdown-bitwarden-use-password-tooltip = Accedi con la tua email e la password principale
dropdown-bitwarden-two-factor-title = Accesso in due passaggi
dropdown-bitwarden-two-factor-app = Inserisci il codice della tua app di autenticazione.
dropdown-bitwarden-two-factor-email = Inserisci il codice inviato via email a { $email }.
dropdown-bitwarden-new-device-title = Nuovo dispositivo
dropdown-bitwarden-new-device-desc = Bitwarden ha inviato un codice di verifica a { $email } per confermare questo dispositivo.
dropdown-bitwarden-code-placeholder = Codice
dropdown-bitwarden-send-code = Invia codice
dropdown-bitwarden-send-code-tooltip = Invia il codice di verifica all'indirizzo email del tuo account
dropdown-bitwarden-back = Indietro
dropdown-bitwarden-back-login-tooltip = Torna all'accesso
dropdown-bitwarden-back-vault-tooltip = Torna alla cassaforte
dropdown-bitwarden-verify = Verifica
dropdown-bitwarden-verify-tooltip = Controlla il codice e completa l'accesso
dropdown-bitwarden-unlock = Sblocca
dropdown-bitwarden-unlock-tooltip = Apri la cassaforte con la tua password principale
dropdown-bitwarden-sync = Sincronizza
dropdown-bitwarden-lock = Blocca
dropdown-bitwarden-logout = Esci
dropdown-bitwarden-search = Cerca nella cassaforte
dropdown-bitwarden-no-items = Nessun elemento
dropdown-bitwarden-more = Mostrati { $shown } di { $total } elementi. Cerca per restringere i risultati.
dropdown-bitwarden-undecryptable = Impossibile decifrare
dropdown-bitwarden-copy = Copia
dropdown-bitwarden-copy-reprompt-tooltip = Verifica la password principale e copia il campo protetto
dropdown-bitwarden-cancel = Annulla
dropdown-bitwarden-cancel-tooltip = Non copiare nulla e torna alla cassaforte
dropdown-bitwarden-copy-field = Copia { $field }
dropdown-bitwarden-copied = Copiato: { $field }.
dropdown-bitwarden-copied-clears = Copiato: { $field }. Gli appunti si svuotano tra { $seconds } s.
dropdown-bitwarden-reprompt-title = Richiesta della password principale
dropdown-bitwarden-reprompt-desc = { $item } è protetto: inserisci la tua password principale per copiarne il campo { $field }.
dropdown-bitwarden-field-username = nome utente
dropdown-bitwarden-field-password = password
dropdown-bitwarden-field-totp = codice di verifica
dropdown-bitwarden-field-card-number = numero della carta
dropdown-bitwarden-field-card-code = codice di sicurezza
dropdown-bitwarden-field-email = email
dropdown-bitwarden-field-notes = nota
dropdown-bitwarden-field-passphrase = frase segreta
dropdown-bitwarden-generator = Genera una password
dropdown-bitwarden-generate = Genera e copia un nuovo valore per { $field }
dropdown-bitwarden-generator-password-caption = { $length } caratteri: { $charsets }
dropdown-bitwarden-generator-passphrase-caption = { $words } parole
dropdown-bitwarden-generator-empty = Nessun preset del generatore
dropdown-bitwarden-generator-empty-hint = Aggiungili in modules.bitwarden.generator-passwords e generator-passphrases.
dropdown-bitwarden-error-wrong-password = Password principale errata.
dropdown-bitwarden-error-credentials = L'email, la password principale o il codice non sono corretti.
dropdown-bitwarden-error-login-required = La sessione è terminata. Accedi di nuovo.
dropdown-bitwarden-error-unavailable = Il server non è disponibile. Riprova più tardi.
dropdown-bitwarden-error-server = Il server ha rifiutato: { $message }
dropdown-bitwarden-error-network = Impossibile raggiungere il server.
launcher-bitwarden-locked = Bitwarden è bloccato. Digita la tua password principale e premi Invio.
launcher-bitwarden-logged-out = Accedi prima a Bitwarden dal suo menu a discesa nella barra.
launcher-bitwarden-hint = Invio copia la password o il primo campo di un elemento; Maiusc+Invio ne elenca i campi.
dropdown-bitwarden-totp-live = { $code } · { $seconds } s rimanenti
dropdown-bitwarden-notify-sync-failed = Bitwarden non è riuscito a sincronizzare
dropdown-bitwarden-notify-logged-out = Bitwarden ti ha disconnesso
dropdown-bitwarden-notify-logged-out-body = La sessione è scaduta o è stata revocata. Accedi di nuovo dalla barra.
dropdown-bitwarden-sends = Bitwarden Send
dropdown-bitwarden-send-name = Nome
dropdown-bitwarden-send-name-placeholder = Cosa stai inviando
dropdown-bitwarden-send-text = Testo
dropdown-bitwarden-send-text-placeholder = Il testo che il link apre
dropdown-bitwarden-send-deletion = Elimina dopo
dropdown-bitwarden-send-max-access = Aperture massime (0 per nessun limite)
dropdown-bitwarden-send-password = Password
dropdown-bitwarden-send-password-placeholder = Facoltativa, richiesta prima di mostrare il testo
dropdown-bitwarden-send-create = Crea e copia il link
dropdown-bitwarden-send-create-tooltip = Crea il Send e copia il suo link di condivisione
dropdown-bitwarden-send-none = Nessun Send
dropdown-bitwarden-send-delete = Elimina { $name }
dropdown-bitwarden-send-hours = { $hours ->
    [one] 1 ora
    [many] { $hours } ore
   *[other] { $hours } ore
}
dropdown-bitwarden-send-days = { $days ->
    [one] 1 giorno
    [many] { $days } giorni
   *[other] { $days } giorni
}
dropdown-bitwarden-send-accesses = { $count ->
    [one] aperto una volta
    [many] aperto { $count } volte
   *[other] aperto { $count } volte
}
dropdown-bitwarden-send-accesses-of = aperto { $count } di { $max } volte
dropdown-bitwarden-send-deletes = eliminato { $date }
dropdown-bitwarden-send-protected = password
dropdown-bitwarden-send-disabled = disattivato
dropdown-bitwarden-field-send-link = link del Send
