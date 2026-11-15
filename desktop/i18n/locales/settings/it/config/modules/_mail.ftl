### lys-shell Configuration - Mail Module

## Mail Module Configuration

settings-modules-mail-format = Formato
    .description = Stringa di formato dell'etichetta. Segnaposto: {"{{ count }}"}

settings-modules-mail-query = Query
    .description = Query di ricerca notmuch di cui viene mostrato il numero di corrispondenze (ad es. tag:unread)

settings-modules-mail-hide-when-zero = Nascondi quando zero
    .description = Nasconde completamente il modulo mentre il conteggio è zero

settings-modules-mail-accounts = Account
    .description = Dettaglio dei non letti per account mostrato nel menu a discesa; il conteggio nella barra è la loro somma

enum-mail-provider-generic = Generico
enum-mail-provider-gmail = Gmail
enum-mail-provider-outlook = Outlook
enum-mail-provider-icloud = iCloud
enum-mail-provider-proton = Proton
enum-mail-provider-fastmail = Fastmail
enum-mail-provider-yahoo = Yahoo

settings-modules-mail-notify = Notifica la nuova posta
    .description = Invia una notifica desktop per ogni nuovo messaggio (mittente, oggetto, icona del provider)

settings-modules-mail-notify-summary = Riepilogo della notifica
    .description = Testo del riepilogo. Segnaposto: {"{{ sender }}"}, {"{{ subject }}"}, {"{{ count }}"} (totale), {"{{ new }}"} (nuovi)

settings-modules-mail-notify-body = Corpo della notifica
    .description = Testo del corpo. Segnaposto: {"{{ sender }}"}, {"{{ subject }}"}, {"{{ count }}"} (totale), {"{{ new }}"} (nuovi)

settings-modules-mail-icon-name = Icona
    .description = Icona del modulo

settings-modules-mail-border-show = Mostra il bordo
    .description = Mostra un bordo attorno al pulsante

settings-modules-mail-border-color = Colore del bordo
    .description = Token del colore del bordo

settings-modules-mail-icon-show = Mostra l'icona
    .description = Mostra l'icona del modulo

settings-modules-mail-tooltip = Mostra il suggerimento
    .description = Mostra lo stato del modulo in un suggerimento al passaggio del mouse

settings-modules-mail-icon-color = Colore dell'icona
    .description = Colore di primo piano dell'icona

settings-modules-mail-icon-bg-color = Sfondo dell'icona
    .description = Colore di sfondo del contenitore dell'icona

settings-modules-mail-label-show = Mostra l'etichetta
    .description = Mostra l'etichetta

settings-modules-mail-label-color = Colore dell'etichetta
    .description = Colore del testo dell'etichetta

settings-modules-mail-label-max-length = Lunghezza massima dell'etichetta
    .description = Numero massimo di caratteri prima del troncamento

settings-modules-mail-button-bg-color = Sfondo del pulsante
    .description = Colore di sfondo del pulsante

settings-modules-mail-left-click = Clic sinistro
    .description = Azione al clic sinistro. Vuoto per nessuna azione, oppure un comando della shell (ad es. il tuo client di posta)

settings-modules-mail-right-click = Clic destro
    .description = Comando della shell al clic destro

settings-modules-mail-middle-click = Clic centrale
    .description = Comando della shell al clic centrale

settings-modules-mail-scroll-up = Scorrimento in alto
    .description = Comando della shell allo scorrimento in alto

settings-modules-mail-scroll-down = Scorrimento in basso
    .description = Comando della shell allo scorrimento in basso

## Format placeholders

settings-modules-mail-format-placeholder-count = Numero di messaggi corrispondenti alla query

settings-modules-mail-notify-summary-placeholder-sender = Mittente del messaggio (nome o indirizzo)

settings-modules-mail-notify-summary-placeholder-subject = Oggetto del messaggio

settings-modules-mail-notify-summary-placeholder-count = Totale dei messaggi corrispondenti alla query

settings-modules-mail-notify-summary-placeholder-new = Quanti ne sono arrivati dall'ultimo conteggio

settings-modules-mail-notify-body-placeholder-sender = Mittente del messaggio (nome o indirizzo)

settings-modules-mail-notify-body-placeholder-subject = Oggetto del messaggio

settings-modules-mail-notify-body-placeholder-count = Totale dei messaggi corrispondenti alla query

settings-modules-mail-notify-body-placeholder-new = Quanti ne sono arrivati dall'ultimo conteggio
