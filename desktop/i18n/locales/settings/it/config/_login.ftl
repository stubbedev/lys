### lys-shell Configuration - Login Screen

## Login screen: the greeter and the lock screen

settings-login-background-mode = Sfondo
    .description = Come viene disegnato lo sfondo della schermata di accesso: colore pieno, un'immagine o lo sfondo del desktop

settings-login-background-image = Immagine di sfondo
    .description = File immagine mostrato dietro la schermata di accesso quando lo sfondo è impostato su "image"

settings-login-background-color = Colore di sfondo
    .description = Colore di riempimento mostrato dietro la schermata di accesso quando lo sfondo è impostato su "color"

settings-login-blur = Sfocatura
    .description = Raggio della sfocatura gaussiana applicata agli sfondi immagine o desktop (0 = nessuna)

settings-login-show-clock = Mostra l'orologio
    .description = Mostra un orologio sopra la richiesta delle credenziali

settings-login-clock-format = Formato dell'orologio
    .description = Formato strftime per l'ora della schermata di accesso (ad es. %H:%M)

settings-login-date-format = Formato della data
    .description = Formato strftime per la data della schermata di accesso (ad es. %A %-d %B)

settings-login-max-attempts = Tentativi massimi
    .description = Tentativi di password falliti nella schermata di blocco prima che l'inserimento venga bloccato (0 = illimitati). Lo schermo resta bloccato.

settings-login-show-failed-attempts = Mostra i tentativi falliti
    .description = Mostra il numero di tentativi falliti nella schermata di blocco

settings-login-lock-enabled = Attivato
    .description = Consente a lys-shell di bloccare la sessione (risponde a loginctl lock-session e a `lys-shell lock`)

settings-login-lock-lock-on-start = Blocca all'avvio
    .description = Blocca la sessione appena si avvia la shell, per le configurazioni con accesso automatico in cui la schermata di blocco fa da controllo d'accesso

settings-login-lock-grace-period-ms = Periodo di tolleranza
    .description = Intervallo dopo il blocco in cui lo schermo si sblocca senza password (ms, 0 = richiedila sempre)

settings-login-lock-blank-timeout-ms = Oscuramento dopo
    .description = Oscura la schermata di blocco dopo questo tempo di inattività (ms, 0 = mai; solo visivo: il DPMS è compito del demone di inattività)

settings-login-lock-pam-service = Servizio PAM
    .description = Servizio PAM usato per verificare la password (ad es. system-auth, login)

settings-login-lock-show-media = Mostra il brano in riproduzione
    .description = Mostra il brano in riproduzione con riproduci/pausa e successivo nella schermata di blocco (solo copertine locali)

settings-login-lock-show-battery = Mostra la batteria
    .description = Mostra la carica della batteria nella schermata di blocco; assente senza batteria

settings-login-lock-fingerprint = Impronta digitale
    .description = Sblocca con un'impronta registrata (fprintd) oltre alla password

settings-login-lock-show-switch-user = Mostra Cambia utente
    .description = Offre «Cambia utente» nella schermata di blocco quando il computer ha più di un account

settings-login-greeter-show-user-list = Mostra l'elenco utenti
    .description = Mostra gli avatar cliccabili degli utenti del computer sopra il campo del nome utente

settings-login-greeter-show-power-buttons = Mostra i pulsanti di alimentazione
    .description = Mostra i pulsanti di spegnimento e riavvio in fondo alla schermata di accesso

settings-login-greeter-fingerprint = Accesso con impronta
    .description = Accedere con un'impronta registrata (fprintd) oltre che con la password

settings-login-greeter-cursor-theme = Tema del cursore
    .description = Tema Xcursor usato nella schermata di accesso (vuoto = predefinito di sistema)

settings-login-greeter-cursor-size = Dimensione del cursore
    .description = Dimensione logica del cursore; scalata per schermo così le uscite HiDPI hanno un cursore nitido

settings-login-greeter-screen-reader = Lettore dello schermo
    .description = Lettore dello schermo avviato dal menu accessibilità della schermata di accesso e da Super+Alt+S; nascosto se non installato

## LockBackground variants
enum-lock-background-color = Colore
enum-lock-background-image = Immagine
enum-lock-background-wallpaper = Sfondo del desktop
