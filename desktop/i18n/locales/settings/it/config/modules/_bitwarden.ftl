### lys-shell Configuration - Bitwarden Module

## Bitwarden Module Configuration

settings-modules-bitwarden-server = Server
    .description = bitwarden.com, bitwarden.eu o l'URL di base di un server self-hosted. Un account connesso mantiene il server a cui ha effettuato l'accesso

settings-modules-bitwarden-email = Email
    .description = Email con cui si apre il modulo di accesso

settings-modules-bitwarden-lock-with-session = Blocca con la sessione
    .description = Blocca la cassaforte quando si blocca la sessione

settings-modules-bitwarden-unlock-with-session = Sblocca con la sessione
    .description = Sblocca una cassaforte bloccata con la password della schermata di blocco, quando coincide con la password principale

settings-modules-bitwarden-clipboard-clear = Svuota gli appunti dopo (secondi)
    .description = Cancella un segreto copiato dopo questo numero di secondi, a meno che non sia stato copiato altro. 0 lo lascia

settings-modules-bitwarden-network-folder = Cartella di rete
    .description = Cartella della cassaforte i cui login, con lo stesso nome della connessione, rispondono alle richieste di password Wi-Fi e VPN mentre la cassaforte è aperta. Vuoto non risponde a nessuna

settings-modules-bitwarden-portal-folder = Cartella dei segreti del portale
    .description = Cartella della cassaforte in cui vengono conservate le chiavi del portachiavi delle app in sandbox, dopo averle autorizzate una per una. Vuoto le conserva solo nel portachiavi

settings-modules-bitwarden-generator-passwords = Preset di password
    .description = Password di caratteri casuali che il generatore del menu a discesa copia

settings-modules-bitwarden-generator-passphrases = Preset di frasi segrete
    .description = Frasi segrete composte da parole del grande elenco EFF che il generatore del menu a discesa copia

enum-password-charset-lowercase = Minuscole (a-z)
enum-password-charset-uppercase = Maiuscole (A-Z)
enum-password-charset-digits = Cifre (0-9)
enum-password-charset-symbols = Simboli (!@#$%^&*)

settings-modules-bitwarden-format = Formato
    .description = Stringa di formato dell'etichetta. Segnaposto: {"{{ count }}"} (elementi nella cassaforte aperta)

settings-modules-bitwarden-icon-locked = Icona bloccata
    .description = Icona quando la cassaforte è bloccata o disconnessa

settings-modules-bitwarden-icon-unlocked = Icona sbloccata
    .description = Icona quando la cassaforte è aperta

settings-modules-bitwarden-border-show = Mostra il bordo
    .description = Mostra un bordo attorno al pulsante

settings-modules-bitwarden-border-color = Colore del bordo
    .description = Token del colore del bordo

settings-modules-bitwarden-icon-show = Mostra l'icona
    .description = Mostra l'icona del modulo

settings-modules-bitwarden-tooltip = Mostra il suggerimento
    .description = Mostra lo stato del modulo in un suggerimento al passaggio del mouse

settings-modules-bitwarden-icon-color = Colore dell'icona
    .description = Colore di primo piano dell'icona

settings-modules-bitwarden-icon-bg-color = Sfondo dell'icona
    .description = Colore di sfondo del contenitore dell'icona

settings-modules-bitwarden-label-show = Mostra l'etichetta
    .description = Mostra l'etichetta

settings-modules-bitwarden-label-color = Colore dell'etichetta
    .description = Colore del testo dell'etichetta

settings-modules-bitwarden-label-max-length = Lunghezza massima dell'etichetta
    .description = Numero massimo di caratteri prima del troncamento

settings-modules-bitwarden-button-bg-color = Sfondo del pulsante
    .description = Colore di sfondo del pulsante

settings-modules-bitwarden-left-click = Clic sinistro
    .description = Azione al clic sinistro. Per impostazione predefinita apre il menu a discesa della cassaforte

settings-modules-bitwarden-right-click = Clic destro
    .description = Comando della shell al clic destro

settings-modules-bitwarden-middle-click = Clic centrale
    .description = Comando della shell al clic centrale

settings-modules-bitwarden-scroll-up = Scorrimento in alto
    .description = Comando della shell allo scorrimento in alto

settings-modules-bitwarden-scroll-down = Scorrimento in basso
    .description = Comando della shell allo scorrimento in basso

## Format placeholders

settings-modules-bitwarden-format-placeholder-count = Numero di elementi nella cassaforte aperta (vuoto quando bloccata)
