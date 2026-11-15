### lys-shell Configuration - Notification Module Settings

## Shared Settings

settings-modules-notifications-blocklist = Elenco di blocco
    .description = Pattern glob dei nomi delle app le cui notifiche sono bloccate

settings-modules-notifications-icon-source = Origine delle icone
    .description = Come vengono risolte le icone delle notifiche

settings-modules-notifications-history-memory = Cronologia in memoria
    .description = Notifiche più recenti conservate in memoria; quelle più vecchie vengono caricate dal disco scorrendo

settings-modules-notifications-history-stored = Cronologia su disco
    .description = Notifiche conservate su disco; oltre questo numero le più vecchie vengono eliminate

## Popup Settings

settings-modules-notifications-popup-position = Posizione dei popup
    .description = Angolo dello schermo in cui compaiono i popup delle notifiche

settings-modules-notifications-popup-max-visible = Massimo visibili
    .description = Numero massimo di popup mostrati contemporaneamente prima di impilarli

settings-modules-notifications-popup-stacking-order = Ordine di impilamento
    .description = Se i nuovi popup compaiono sopra o sotto quelli esistenti

settings-modules-notifications-popup-stack = Impilamento
    .description = Quali popup si raccolgono in una pila che si apre a ventaglio al passaggio del mouse

settings-modules-notifications-popup-opacity = Opacità a riposo
    .description = Opacità della superficie smerigliata di un popup quando il puntatore è altrove

settings-modules-notifications-popup-margin-x = Margine orizzontale
    .description = Distanza dal bordo sinistro/destro dello schermo

settings-modules-notifications-popup-margin-y = Margine verticale
    .description = Distanza dal bordo superiore/inferiore dello schermo

settings-modules-notifications-popup-gap = Distanza tra i popup
    .description = Spaziatura tra i popup delle notifiche impilati

settings-modules-notifications-popup-monitor = Monitor dei popup
    .description = Quale monitor mostra i popup: "primary" o un connettore come "DP-1"

settings-modules-notifications-popup-layer = Livello dei popup
    .description = Livello layer-shell su cui vengono posti i popup delle notifiche. La modalità tearing declassa overlay a top.

settings-modules-notifications-popup-duration = Chiusura automatica
    .description = Per quanto restano visibili i popup prima di chiudersi (ms)

settings-modules-notifications-popup-hover-pause = Pausa al passaggio del mouse
    .description = Ferma il timer di chiusura automatica mentre il puntatore è su un popup

settings-modules-notifications-popup-close-behavior = Comportamento alla chiusura
    .description = Cosa succede quando chiudi un popup (ignora o segna come letto)

settings-modules-notifications-popup-urgency-bar = Barra di urgenza
    .description = Mostra una barra colorata sui popup che raggiungono un livello minimo di urgenza

## Relative Time

notification-popup-time-just-now = Adesso
notification-popup-time-minutes-ago = { $minutes } min fa
notification-popup-time-hours-ago = { $hours } h fa

settings-modules-notifications-sounds = Suoni
    .description = Quali notifiche riproducono un suono; silenziose in Non disturbare, durante la condivisione dello schermo e con l'audio disattivato

settings-modules-notifications-sound-default = Suono predefinito
    .description = Nome del tema sonoro riprodotto da una notifica senza un proprio suono

settings-modules-notifications-sound-mute-apps = App silenziose
    .description = Glob dei nomi delle app le cui notifiche non riproducono mai un suono
settings-modules-notifications-sound-volume = Volume dei suoni
    .description = Volume dei suoni delle notifiche, come frazione del suono registrato
