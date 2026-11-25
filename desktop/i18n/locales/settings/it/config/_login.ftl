### lys-shell Configuration - Login Screen

## Login screen: the greeter and the lock screen

settings-login-background-mode = Sfondo
    .description = Come viene disegnato lo sfondo della schermata di accesso: colore pieno, un'immagine o lo sfondo del desktop

settings-login-background-image = Immagine di sfondo
    .description = Immagine mostrata dietro la schermata di accesso quando lo sfondo è Immagine

settings-login-background-color = Colore di sfondo
    .description = Colore di riempimento dietro la schermata di accesso quando lo sfondo è Colore

settings-login-blur = Sfocatura
    .description = Quanto è sfocato uno sfondo immagine o desktop; 0 lo lascia nitido

settings-login-show-clock = Mostra l'orologio
    .description = Mostra un orologio sopra la richiesta delle credenziali

settings-login-clock-format = Formato dell'orologio
    .description = Come appare l'ora nella schermata di accesso

settings-login-date-format = Formato della data
    .description = Come appare la data nella schermata di accesso

settings-login-max-attempts = Tentativi massimi
    .description = Password errate nella schermata di blocco prima che la digitazione venga bloccata, con lo schermo ancora bloccato; 0 non pone limiti

settings-login-show-failed-attempts = Mostra i tentativi falliti
    .description = Mostra il numero di tentativi falliti nella schermata di blocco

settings-login-lock-enabled = Attivato
    .description = Consente a lys-shell di bloccare la sessione (risponde a loginctl lock-session e a lys-shell lock)

settings-login-lock-lock-on-start = Blocca all'avvio
    .description = Blocca la sessione appena si avvia la shell, per le configurazioni con accesso automatico in cui la schermata di blocco fa da controllo d'accesso

settings-login-lock-grace-period-ms = Periodo di tolleranza
    .description = Per quanto tempo dopo il blocco lo schermo si sblocca ancora senza password

settings-login-lock-blank-timeout-ms = Oscuramento dopo
    .description = Oscura la schermata di blocco dopo questo tempo di inattività; quando si spengono gli schermi si imposta nella pagina Inattività e alimentazione

settings-login-lock-pam-service = Servizio PAM
    .description = Servizio PAM che verifica la password, come system-auth o login

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

settings-login-greeter-match-cursor = Usa il mio cursore
    .description = Mostra il cursore dell'ultima sessione avviata invece del tema e della dimensione impostati qui

settings-login-greeter-cursor-theme = Tema del cursore
    .description = Tema del cursore nella schermata di accesso; se non impostato, quello di sistema

settings-login-greeter-cursor-size = Dimensione del cursore
    .description = Dimensione logica del cursore; scalata per schermo così le uscite HiDPI hanno un cursore nitido

settings-login-greeter-screen-reader = Lettore dello schermo
    .description = Lettore dello schermo avviato dal menu accessibilità della schermata di accesso e da Super+Alt+S; nascosto se non installato

## LockBackground variants
enum-lock-background-color = Colore
enum-lock-background-image = Immagine
enum-lock-background-wallpaper = Sfondo del desktop
