### lys-shell Configuration - Notifications Module

## Notifications Module Configuration

settings-modules-notifications-enabled = Attivato
    .description = Attiva il servizio e il modulo delle notifiche

settings-modules-notifications-icon-name = Icona predefinita
    .description = Icona mostrata quando non ci sono notifiche

settings-modules-notifications-icon-unread = Icona non lette
    .description = Icona mostrata quando ci sono notifiche

settings-modules-notifications-icon-dnd = Icona Non disturbare
    .description = Icona mostrata quando Non disturbare è attivo

settings-modules-notifications-border-show = Mostra il bordo
    .description = Mostra un bordo attorno al pulsante

settings-modules-notifications-border-color = Colore del bordo
    .description = Token del colore del bordo

settings-modules-notifications-icon-show = Mostra l'icona
    .description = Mostra l'icona del modulo

settings-modules-notifications-tooltip = Mostra il suggerimento
    .description = Mostra lo stato del modulo in un suggerimento al passaggio del mouse

settings-modules-notifications-icon-color = Colore dell'icona
    .description = Colore di primo piano dell'icona

settings-modules-notifications-icon-bg-color = Sfondo dell'icona
    .description = Colore di sfondo del contenitore dell'icona

settings-modules-notifications-label-show = Mostra l'etichetta
    .description = Mostra l'etichetta con il numero di notifiche

settings-modules-notifications-label-color = Colore dell'etichetta
    .description = Colore del testo dell'etichetta

settings-modules-notifications-label-max-length = Lunghezza massima dell'etichetta
    .description = Numero massimo di caratteri prima del troncamento

settings-modules-notifications-button-bg-color = Sfondo del pulsante
    .description = Colore di sfondo del pulsante

settings-modules-notifications-middle-click = Clic centrale
    .description = Comando della shell al clic centrale

settings-modules-notifications-scroll-up = Scorrimento in alto
    .description = Comando della shell allo scorrimento in alto

settings-modules-notifications-scroll-down = Scorrimento in basso
    .description = Comando della shell allo scorrimento in basso

settings-modules-notifications-left-click = Clic sinistro
    .description = Azione al clic sinistro

settings-modules-notifications-right-click = Clic destro
    .description = Azione al clic destro

settings-modules-notifications-popup-shadow = Ombra dei popup
    .description = Disegna un'ombra dietro i popup delle notifiche

settings-modules-notifications-thresholds = Soglie
    .description = Soglie di colore che cambiano in base ai livelli del valore


## PopupPosition variants
enum-popup-position-top-left = In alto a sinistra
enum-popup-position-top-center = In alto al centro
enum-popup-position-top-right = In alto a destra
enum-popup-position-bottom-left = In basso a sinistra
enum-popup-position-bottom-center = In basso al centro
enum-popup-position-bottom-right = In basso a destra
enum-popup-position-center-left = Al centro a sinistra
enum-popup-position-center-right = Al centro a destra

## StackingOrder variants
enum-stacking-order-newest-first = Prima le più recenti
enum-stacking-order-oldest-first = Prima le meno recenti

## PopupStack variants
enum-popup-stack-per-app = Per app
enum-popup-stack-all = Tutte
enum-popup-stack-off = Disattivato

## PopupCloseBehavior variants
enum-popup-close-behavior-dismiss = Ignora
enum-popup-close-behavior-remove = Rimuovi

## UrgencyBarThreshold variants
enum-urgency-bar-threshold-low = Bassa
enum-urgency-bar-threshold-normal = Normale
enum-urgency-bar-threshold-critical = Critica
enum-urgency-bar-threshold-none = Nessuna

## IconSource variants
enum-icon-source-automatic = Automatica
enum-icon-source-mapped = Associata
enum-icon-source-application = Applicazione

## NotificationSounds variants
enum-notification-sounds-off = Disattivato
enum-notification-sounds-critical = Solo critiche
enum-notification-sounds-all = Tutte

## Do Not Disturb and Focus

settings-modules-notifications-dnd-allow-critical = Consenti critiche
    .description = Le notifiche critiche, come la batteria scarica o una sveglia, compaiono anche in modalità Non disturbare

settings-modules-notifications-dnd-allow-apps = App consentite
    .description = Nomi di app o voci desktop, con i glob * e ?, le cui notifiche compaiono anche in modalità Non disturbare

settings-modules-notifications-dnd-schedule = Pianificazione
    .description = Orari della settimana in cui Non disturbare si attiva da solo; un intervallo che finisce prima di iniziare prosegue oltre la mezzanotte

settings-modules-notifications-dnd-while-sharing = Durante la condivisione dello schermo
    .description = Attiva Non disturbare mentre lo schermo è condiviso o registrato

settings-modules-notifications-dnd-while-fullscreen = A schermo intero
    .description = Attiva Non disturbare mentre una finestra è a schermo intero
