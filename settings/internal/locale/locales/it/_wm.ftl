### lys-settings: the lys-wm pages

settings-nav-wm-windows = Finestre
settings-page-wm-windows = Finestre
settings-nav-wm-effects = Ombra e sfocatura
settings-page-wm-effects = Ombra e sfocatura
settings-nav-wm-tiling = Affiancamento e schede
settings-page-wm-tiling = Affiancamento e schede
settings-nav-wm-binds = Scorciatoie da tastiera
settings-page-wm-binds = Scorciatoie da tastiera
settings-nav-wm-outputs = Schermi
settings-page-wm-outputs = Schermi
settings-nav-wm-session = Regole e sessione
settings-page-wm-session = Regole e sessione

wm-section-gaps = Spazi
    .description = Spazio attorno e tra le finestre affiancate
wm-section-borders = Bordi
    .description = La linea disegnata attorno a ogni finestra
wm-section-shape = Forma e trasparenza
    .description = Arrotondamento degli angoli e opacità delle finestre in base al focus
wm-section-shadow = Ombra
    .description = L'ombra sotto le finestre
wm-section-blur = Sfocatura
    .description = Cosa si vede attraverso le finestre traslucide
wm-section-overview = Panoramica
    .description = La vista di tutti gli spazi di lavoro insieme
wm-section-drag = Anteprima di trascinamento
    .description = La finestra che segue il puntatore durante il trascinamento
wm-section-autotile = Affiancamento automatico
    .description = Dove si apre una nuova finestra in una disposizione affiancata
wm-section-tabs = Schede
    .description = La barra sopra i gruppi a schede
wm-section-keyboard = Tastiera
    .description = Disposizione e ripetizione dei tasti
wm-section-pointer = Mouse
    .description = Mouse e trackball
wm-section-touchpad = Touchpad
    .description = Tocco, scorrimento e velocità
wm-section-trackpoint = Trackpoint
    .description = Levette di puntamento nella tastiera di un portatile
wm-section-binds = Tasti e gesti
    .description = Combinazioni di tasti e mouse e le azioni che eseguono
wm-section-outputs = Uscite
    .description = Modalità, scala, posizione e colore di ogni schermo
wm-section-display = Schermo
    .description = Impostazioni condivise da tutti gli schermi
wm-section-rules = Regole delle finestre
    .description = Come si aprono e appaiono le finestre corrispondenti
wm-section-layer-rules = Pannelli e sovrapposizioni
    .description = Come appaiono la barra, il dock, i popup e le altre superfici della shell
wm-rules-namespace = Superficie
wm-rules-blur-alpha-threshold = Soglia alfa della sfocatura
wm-layer-lys-shell-bar = Barra
wm-layer-lys-shell-dock = Dock
wm-layer-lys-shell-wallpaper = Sfondo
wm-layer-lys-shell-launcher = Avviatore
wm-layer-lys-shell-notification-popup = Popup delle notifiche
wm-layer-lys-shell-osd = Indicatore di volume e luminosità
wm-layer-lys-shell-power-menu = Menu di spegnimento
wm-layer-lys-shell-portal-dialog = Dialoghi del portale
wm-layer-lys-shell-file-chooser = Selettore di file
wm-layer-lys-shell-print = Dialogo di stampa
wm-layer-lys-shell-share-picker = Selettore di condivisione dello schermo
wm-layer-lys-shell-screenshot-reading = Riconoscimento del testo
wm-layer-lys-shell-screenshot-thumbnail = Miniatura della schermata
wm-layer-lys-shell-color-picker = Selettore di colore
wm-layer-lys-shell-region-overlay = Selezione dell’area
wm-layer-lys-shell-window-pick = Selettore di finestra
wm-section-start = Avvio
    .description = Cosa viene eseguito con la sessione, e il suo ambiente
wm-section-xwayland = Xwayland
    .description = Applicazioni X11
wm-section-palette = Tavolozza
    .description = Nomi di colori utilizzabili dalle altre chiavi

enum-wm-toggle-on = Attivo
enum-wm-toggle-off = Disattivo
wm-status-not-running = Salvato. Nessun lys-wm è in esecuzione qui; il prossimo lo leggerà.
wm-status-unreadable = Impossibile leggere la configurazione di lys-wm
wm-status-read-only = La configurazione di lys-wm non può essere modificata qui, quindi le modifiche non vengono salvate
wm-status-not-applied = lys-wm non ha accettato la modifica
wm-column-widths-unreadable = Le larghezze delle colonne non sono accettate da lys-wm; modificale come testo
wm-pointers-connected = Collegati: { $devices }
wm-pointers-device = { $name } ({ $kind })
wm-pointers-class-pointer = mouse
wm-pointers-class-touchpad = touchpad
wm-pointers-class-trackpoint = trackpoint
wm-pointers-none = Nessun mouse, touchpad o trackpoint è collegato.
wm-pointers-not-running = Nessun lys-wm è in esecuzione qui per dire quali dispositivi sono collegati, quindi vengono mostrate tutte le sezioni.
wm-pointers-outdated = Il lys-wm in esecuzione è più vecchio e non dice quali dispositivi sono collegati, quindi tutte le sezioni restano visibili fino al prossimo accesso.
wm-pointers-unreadable = Non è stato possibile leggere quali dispositivi sono collegati, quindi vengono mostrate tutte le sezioni
wm-section-devices = Singoli dispositivi
    .description = Impostazioni che un dispositivo tiene per sé
wm-input-devices = Ogni dispositivo di puntamento
    .description = Le impostazioni proprie di un dispositivo prevalgono su quelle del suo tipo qui sopra; ciò che lascia vuoto le segue.
wm-input-devices-keyboard = Ogni tastiera
    .description = Il layout proprio di una tastiera prevale su quello qui sopra; ciò che lascia vuoto lo segue.
wm-device-absent = Non collegato
    .description = Questo dispositivo ha impostazioni proprie; tornano a valere quando viene collegato.
wm-device-forget = Dimentica le impostazioni di questo dispositivo
wm-devices-none = Nessun dispositivo è collegato o ha impostazioni proprie.
wm-devices-not-running = Qui non è in esecuzione nessun lys-wm che dica quali dispositivi sono collegati; sono elencati i dispositivi con impostazioni proprie.
wm-devices-outdated = Il lys-wm in esecuzione è più vecchio e non dice quali dispositivi sono collegati; sono elencati i dispositivi con impostazioni proprie fino al prossimo accesso.
wm-devices-unreadable = Non è stato possibile leggere quali dispositivi sono collegati, quindi sono elencati solo i dispositivi con impostazioni proprie
wm-devices-reload = Cerca di nuovo i dispositivi
wm-rules-pick = Scegli una finestra…
wm-rules-picking = Fai clic sulla finestra per cui creare una regola. Esc annulla.
wm-rules-pick-none = Il clic non era su una finestra.
wm-rules-pick-gone = La finestra si è chiusa prima di poter essere letta.
wm-rules-pick-unnamed = Quella finestra non ha né un ID applicazione né un titolo a cui corrispondere.
wm-rules-pick-not-running = Nessun lys-wm è in esecuzione qui da cui scegliere una finestra.
wm-rules-pick-failed = Impossibile scegliere la finestra
wm-rules-applies-to = Si applica a
wm-rules-then = Allora
wm-rules-more-conditions = Altre condizioni
wm-rules-fewer-conditions = Meno condizioni
wm-rules-group-placement = Posizionamento
wm-rules-group-appearance = Aspetto
wm-rules-group-behaviour = Comportamento
wm-rules-group-performance = Prestazioni
wm-rules-add-effect = Aggiungi effetto…
wm-rules-no-effects = Questa regola non cambia ancora nulla: aggiungi un effetto.
wm-rules-remove-effect = Rimuovi questo effetto
wm-rules-enabled = Applica questa regola
wm-rules-apps-running = In esecuzione
wm-rules-border = Bordo
wm-rules-blur-look = Aspetto della sfocatura
wm-preview-windows = Anteprima delle finestre
wm-preview-unreadable = L’anteprima non può disegnare { $setting } impostato su { $value }.
wm-source-file = 
    .description = Impostato nel file di configurazione di lys-wm

wm-gaps-inner = Spazio interno
    .description = Pixel tra le finestre affiancate
    .keywords = spaziatura, distanza, margine
wm-gaps-outer = Spazio esterno
    .description = Pixel tra le finestre affiancate e il bordo dello schermo: un numero per ogni lato, o quattro per alto, destra, basso e sinistra
wm-borders-width = Spessore del bordo
    .description = In pixel; 0 non disegna nulla
wm-borders-active = Bordo con focus
    .description = Un colore, oppure più colori e un angolo per una sfumatura
wm-borders-inactive = Bordo senza focus
    .description = Un colore, oppure più colori e un angolo per una sfumatura
wm-decoration-rounding = Raggio degli angoli
    .description = In pixel
wm-decoration-active-opacity = Opacità con focus
    .description = Quanto è opaca la finestra attiva; al 100% nasconde ciò che sta dietro
wm-decoration-inactive-opacity = Opacità senza focus
    .description = Quanto sono opache le finestre non attive
wm-decoration-fullscreen-opacity = Opacità a schermo intero
    .description = Quanto è opaca una finestra a schermo intero
wm-decoration-dim-inactive = Attenua le finestre senza focus
    .description = Scurisce ogni finestra senza focus
wm-decoration-dim-strength = Intensità dell'attenuazione
    .description = Quanto si scuriscono le finestre non attive
wm-decoration-shadow-enabled = Ombre
    .description = Disegna un'ombra sotto le finestre
wm-decoration-shadow-range = Estensione dell'ombra
    .description = In pixel
wm-decoration-shadow-render-power = Sfumatura dell'ombra
    .description = Da 1 a 4; più alto sfuma più rapidamente
wm-decoration-shadow-sharp = Ombre nette
    .description = Senza bordo sfumato
wm-decoration-shadow-scale = Scala dell'ombra
    .description = Maggiore di 0, fino a 1
wm-decoration-shadow-offset = Spostamento dell'ombra
    .description = Quanto l'ombra cade a destra e sotto la finestra, in pixel; un valore negativo la sposta a sinistra o in alto
wm-shadow-offset-x = A destra
wm-shadow-offset-y = In basso
wm-decoration-shadow-color = Colore dell'ombra
    .description = Un colore della tavolozza o uno personalizzato; anche il colore dell'ombra senza focus, se non è impostato
wm-decoration-shadow-color-inactive = Colore dell'ombra senza focus
    .description = Un colore della tavolozza o uno personalizzato
wm-decoration-blur-enabled = Sfocatura
    .description = Sfoca dietro ogni finestra traslucida, non solo quelle che lo richiedono
    .keywords = vetro smerigliato, trasparenza, traslucido
wm-decoration-blur-size = Dimensione della sfocatura
    .description = In pixel
wm-decoration-blur-passes = Passaggi di sfocatura
    .description = Da 1 a 4
wm-decoration-blur-xray = Sfocatura a raggi X
    .description = Sfoca solo lo sfondo, non le finestre sottostanti
wm-decoration-blur-tint = Tinta della sfocatura
    .description = Un colore della tavolozza o uno personalizzato
wm-decoration-blur-saturation = Saturazione della sfocatura
    .description = Da 0 a 4; 1 la lascia invariata
wm-decoration-blur-brightness = Luminosità della sfocatura
    .description = Da 0 a 4; 1 la lascia invariata
wm-decoration-blur-contrast = Contrasto della sfocatura
    .description = Da 0 a 4; 1 la lascia invariata
wm-decoration-blur-grain = Grana della sfocatura
    .description = Rumore sopra la sfocatura; lo 0% non ne aggiunge
wm-animations-enabled = Animazioni
    .description = Anima finestre, spazi di lavoro e livelli
wm-animations-reduced-motion = Movimento ridotto
    .description = Dissolvenza invece del movimento
    .keywords = accessibilità, cinetosi
wm-overview-blur = Sfoca la panoramica
    .description = Sfoca ciò che si trova dietro la panoramica
wm-overview-dim = Attenuazione della panoramica
    .description = Quanto si scurisce il desktop dietro la panoramica
wm-drag-preview-enabled = Anteprima di trascinamento
    .description = Mostra dove finirà una finestra trascinata
wm-drag-preview-opacity = Opacità dell'anteprima
    .description = Quanto è opaca l’anteprima
wm-drag-preview-blur = Sfoca l'anteprima
    .description = Sfoca dietro l'anteprima
wm-drag-preview-snap-distance = Distanza di aggancio
    .description = In pixel da un bordo
wm-autotile-enabled = Affiancamento automatico
    .description = Divide una finestra lungo il lato più lungo
wm-autotile-ephemeral = Gruppi temporanei
    .description = Un gruppo con una sola finestra si scioglie
wm-autotile-min-trigger = Dimensione minima per dividere
    .description = In pixel
wm-autotile-divisor = Divisore di suddivisione
    .description = Le finestre più strette di questa frazione dell'uscita smettono di dividersi
wm-tabs-height = Altezza della barra delle schede
    .description = In pixel
wm-tabs-padding = Spaziatura della barra delle schede
    .description = In pixel
wm-tabs-text-height = Altezza del testo delle schede
    .description = In pixel
wm-tabs-text-padding = Spaziatura del testo delle schede
    .description = In pixel
wm-tabs-font = Carattere delle schede
    .description = Percorso di un file di carattere; vuoto usa quello predefinito
wm-tabs-icon-theme = Tema di icone delle schede
    .description = Vuoto usa quello predefinito
wm-input-keyboard-layout = Disposizioni della tastiera
    .description = Le disposizioni con cui scrivi, nell'ordine in cui si alternano; aggiungine una per nome, varianti come Danese (senza tasti morti) comprese
    .keywords = mappa della tastiera, lingua, sorgente di input
wm-input-sources-misaligned = Queste disposizioni e varianti non si accoppiano, ci sono più varianti che disposizioni, quindi entrambe sono mostrate come scritte; scegli una disposizione o una variante per reimpostarle
wm-input-keyboard-variant = Variante della tastiera
    .description = Variante XKB
wm-input-keyboard-options = Opzioni della tastiera
    .description = Opzioni XKB, ad esempio caps:escape
wm-input-keyboard-model = Modello della tastiera
    .description = Modello XKB
wm-input-keyboard-rules = Regole della tastiera
    .description = Regole XKB
wm-input-keyboard-repeat-rate = Frequenza di ripetizione
    .description = Tasti al secondo
    .keywords = ripetizione dei tasti, typematic
wm-input-keyboard-repeat-delay = Ritardo di ripetizione
    .description = Millisecondi prima che un tasto tenuto premuto si ripeta
wm-input-pointer-natural-scroll = Scorrimento naturale
    .description = Il contenuto segue le dita
    .keywords = scorrimento inverso, scorrimento invertito, rotellina del mouse
wm-input-pointer-middle-emulation = Emulazione del clic centrale
    .description = Sinistro e destro insieme fanno clic con il pulsante centrale
wm-input-pointer-accel-profile = Profilo di accelerazione
    .description = Piatto sposta il puntatore quanto il dispositivo; adattivo lo accelera con movimenti più rapidi
wm-input-pointer-accel-speed = Velocità di accelerazione
    .description = Da -1 a 1
wm-input-pointer-accel-points = Curva di accelerazione personalizzata
    .description = La velocità del puntatore a ogni passo della velocità del dispositivo, al posto del profilo
wm-input-pointer-scroll-factor = Velocità di scorrimento
    .description = Maggiore di 0, fino a 100; 1 la lascia invariata
wm-input-trackpoint-natural-scroll = Scorrimento naturale
    .description = Il contenuto segue le dita
    .keywords = scorrimento inverso, scorrimento invertito, rotellina del mouse
wm-input-trackpoint-middle-emulation = Emulazione del clic centrale
    .description = Sinistro e destro insieme fanno clic con il pulsante centrale
wm-input-trackpoint-accel-profile = Profilo di accelerazione
    .description = Piatto sposta il puntatore quanto il dispositivo; adattivo lo accelera con movimenti più rapidi
wm-input-trackpoint-accel-speed = Velocità di accelerazione
    .description = Da -1 a 1
wm-input-trackpoint-accel-points = Curva di accelerazione personalizzata
    .description = La velocità del puntatore a ogni passo della velocità del dispositivo, al posto del profilo
wm-input-trackpoint-scroll-factor = Velocità di scorrimento
    .description = Maggiore di 0, fino a 100; 1 la lascia invariata
wm-input-trackpoint-scroll-method = Metodo di scorrimento
    .description = Come scorre il trackpoint: con un pulsante tenuto premuto, o per nulla
wm-input-touchpad-natural-scroll = Scorrimento naturale
    .description = Il contenuto segue le dita
    .keywords = scorrimento inverso, scorrimento invertito, trackpad
wm-input-touchpad-middle-emulation = Emulazione del clic centrale
    .description = Sinistro e destro insieme fanno clic con il pulsante centrale
wm-input-touchpad-accel-profile = Profilo di accelerazione
    .description = Piatto sposta il puntatore quanto il dito; adattivo lo accelera con movimenti più rapidi
wm-input-touchpad-accel-speed = Velocità di accelerazione
    .description = Da -1 a 1
wm-input-touchpad-accel-points = Curva di accelerazione personalizzata
    .description = La velocità del puntatore a ogni passo della velocità del dispositivo, al posto del profilo
wm-input-touchpad-scroll-factor = Velocità di scorrimento
    .description = Maggiore di 0, fino a 100; 1 la lascia invariata
wm-input-touchpad-tap = Tocca per fare clic
    .description = Un tocco fa clic, senza premere il touchpad
    .keywords = tocca per fare clic, trackpad
wm-input-touchpad-drag-lock = Blocco del trascinamento
    .description = Un trascinamento con tocco continua anche sollevando il dito
wm-input-touchpad-disable-while-typing = Disattiva durante la digitazione
    .description = Ignora il touchpad per un istante dopo ogni pressione di tasto
wm-input-touchpad-click-method = Metodo di clic
    .description = Come una pressione sceglie il pulsante: in base a dove avviene o a quante dita premono
wm-input-touchpad-scroll-method = Metodo di scorrimento
    .description = Come scorre il touchpad: con due dita, dal bordo o con un pulsante tenuto premuto
wm-binds = Associazioni
    .description = Un elenco di combinazione, azione e flag; una configurazione senza associazioni non ne ha nessuna
wm-submaps = Sottomappe
    .description = Insiemi di associazioni con nome
wm-gestures = Gesti
    .description = Scorrimenti, pizzichi e pressioni prolungate sul touchpad
wm-outputs = Regole delle uscite
    .description = Ogni uscita usa la prima regola che le corrisponde
wm-color-management = Gestione del colore
    .description = Offre ai client il protocollo di gestione del colore
wm-tearing = Tearing
    .description = Consente il tearing a schermo intero alle finestre che lo richiedono
wm-gpu = GPU
    .description = Una scheda o un nodo di rendering da usare; vuoto ne sceglie uno
wm-lid-action = Alla chiusura del coperchio
    .description = Cosa fa la chiusura del coperchio a batteria senza schermi esterni connessi: sospensione, ibernazione, sospensione ibrida, sospensione poi ibernazione, o niente
wm-rules = Regole
    .description = Un elenco di corrispondenza ed effetto
wm-layer-rules = Regole
    .description = Ogni regola cambia l’aspetto delle superfici della shell a cui si applica
wm-autostart = Avvio automatico
    .description = Comandi eseguiti una volta avviata la sessione
wm-env = Ambiente
    .description = Variabili per lys-wm e le app che avvia; una modifica raggiunge le app avviate dopo, non quelle già in esecuzione
wm-env-name-placeholder = Variabile, come GDK_BACKEND
wm-env-name-invalid = Un nome di variabile contiene solo lettere, cifre e _, e non inizia con una cifra; non viene salvato finché non è così
wm-env-moz-wayland = Firefox gira su Wayland
wm-env-qt-wayland = Le app Qt girano su Wayland
wm-env-gdk-wayland = Le app GTK girano su Wayland
wm-env-sdl-wayland = I giochi SDL girano su Wayland
wm-env-electron-wayland = Le app Electron girano su Wayland quando possono
wm-env-java-tiling = Le app Java disegnano correttamente le finestre
wm-xwayland-enabled = Xwayland
    .description = Esegue le applicazioni X11
wm-xwayland-zero-scaling = Scala zero
    .description = Le app X11 disegnano ai pixel reali dello schermo invece di essere ingrandite e sfocate; su uno schermo scalato sembrano piccole se non si scalano da sole
wm-palette = Tavolozza
    .description = Colori con nome utilizzabili dalle altre chiavi

## Structured keys
wm-section-gestures = Gesti
    .description = Gesti del touchpad e cosa controllano
wm-gestures-swipe = Scorrimenti
    .description = Scorrimenti sul touchpad e cosa controlla o esegue ciascuno
wm-gestures-pinch = Pizzichi
    .description = Pizzichi sul touchpad e cosa controlla o esegue ciascuno
wm-gestures-hold = Pressioni prolungate
    .description = Dita tenute sul touchpad e l'azione eseguita da ciascuna
wm-gestures-distance = Distanza del gesto
    .description = Unità del touchpad per uno scorrimento completo, circa 12 per millimetro
wm-card-number = Elemento { $number }
wm-side-top = Alto
wm-side-right = Destra
wm-side-bottom = Basso
wm-side-left = Sinistra
wm-border-add-color = Aggiungi colore
wm-border-angle = Angolo
wm-binds-action = Azione
wm-binds-chord = Combinazione
wm-binds-flags = Flag
wm-binds-cooldown = Intervallo minimo (ms)
wm-binds-description = Descrizione
wm-binds-new = Nuova scorciatoia
wm-binds-long-press = Pressione prolungata (ms)
wm-gestures-action = Azione
wm-gestures-direction = Direzione
wm-gestures-drive = Controlla
wm-gestures-fingers = Dita
wm-gestures-modifiers = Modificatori
wm-gestures-scope = Ambito della panoramica
wm-outputs-bitdepth = Profondità di bit
    .description = 8 o 10 bit per canale; 10 rende più morbide le sfumature con la luce notturna
wm-outputs-disabled = Disattivato
wm-outputs-hdr = HDR
    .description = Usa lo schermo in HDR sempre, oppure solo per i contenuti HDR a schermo intero
wm-outputs-hdr-peak = Luminosità di picco HDR
    .description = Con HDR: la luminosità di picco dello schermo in cd/m², se il suo EDID la riporta in modo errato
wm-outputs-icc = Profilo ICC
    .description = Un profilo ICC matrix-shaper per uno schermo SDR
wm-outputs-hot-corners = Angoli attivi
    .description = Disattivo spegne tutti gli angoli attivi su questo schermo; Attivo ne modifica alcuni qui
wm-outputs-hot-corners-top-left = Angolo in alto a sinistra
wm-outputs-hot-corners-top-left-action = Azione
wm-outputs-hot-corners-top-left-modifiers = Modificatori
wm-outputs-hot-corners-top-right = Angolo in alto a destra
wm-outputs-hot-corners-top-right-action = Azione
wm-outputs-hot-corners-top-right-modifiers = Modificatori
wm-outputs-hot-corners-bottom-left = Angolo in basso a sinistra
wm-outputs-hot-corners-bottom-left-action = Azione
wm-outputs-hot-corners-bottom-left-modifiers = Modificatori
wm-outputs-hot-corners-bottom-right = Angolo in basso a destra
wm-outputs-hot-corners-bottom-right-action = Azione
wm-outputs-hot-corners-bottom-right-modifiers = Modificatori
wm-outputs-match = Corrispondenza
wm-outputs-mirror = Duplica
    .description = Un altro schermo la cui immagine viene mostrata da questo al posto del proprio desktop
wm-outputs-mode = Modalità
wm-outputs-position = Posizione
wm-outputs-reserved = Spazio riservato
    .description = Pixel logici lasciati liberi da finestre affiancate e massimizzate: alto, destra, basso, sinistra
wm-outputs-scale = Scala
wm-outputs-sdr-brightness = Luminosità SDR
    .description = Con HDR: i cd/m² a cui viene mostrato il bianco SDR
wm-outputs-shader = Shader
    .description = Un fragment shader per l'immagine finale dello schermo
wm-outputs-transform = Rotazione
    .description = Come il pannello è ruotato o capovolto
wm-outputs-vrr = Frequenza di aggiornamento variabile
    .description = Sincronizzazione adattiva: disattivata, sempre attiva o solo per un gioco o un video a schermo intero
wm-rules-animation = Animazione
wm-rules-animation-in = Animazione di apertura
wm-rules-animation-out = Animazione di chiusura
wm-rules-app-id = ID dell'app
wm-rules-block-out-from = Nascondi dalle acquisizioni
wm-rules-blur = Sfocatura
wm-rules-blur-look-brightness = Luminosità della sfocatura
wm-rules-blur-look-contrast = Contrasto della sfocatura
wm-rules-blur-look-grain = Grana della sfocatura
wm-rules-blur-look-refraction-amount = Intensità della rifrazione
wm-rules-blur-look-refraction-bezel = Bordo della rifrazione
wm-rules-blur-look-saturation = Saturazione della sfocatura
wm-rules-blur-look-specular-strength = Intensità speculare
wm-rules-blur-look-specular-width = Larghezza speculare
wm-rules-blur-look-tint = Tinta della sfocatura
wm-rules-border-active = Bordo attivo
wm-rules-border-inactive = Bordo inattivo
wm-rules-border-width = Spessore del bordo
wm-rules-dim-around = Attenua attorno
wm-rules-dim-inactive = Attenua quando inattiva
wm-rules-float = Fluttuante
wm-rules-mica = Mica
wm-rules-opacity = Opacità
wm-rules-size = Dimensione
wm-rules-tag = Tag
wm-rules-tearing = Tearing
wm-rules-touch-gestures = Gesti touch
wm-rules-tile = Affianca
wm-rules-vrr = Frequenza di aggiornamento variabile
wm-rules-title = Titolo
wm-rules-exclude-app-id = Tranne ID dell'app
wm-rules-exclude-is-captured = Tranne acquisite
wm-rules-exclude-is-floating = Tranne fluttuanti
wm-rules-exclude-is-focused = Tranne con focus
wm-rules-exclude-is-fullscreen = Tranne a schermo intero
wm-rules-exclude-is-urgent = Tranne urgenti
wm-rules-exclude-initial-app-id = Tranne ID iniziale dell'app
wm-rules-exclude-initial-title = Tranne titolo iniziale
wm-rules-exclude-tag = Tranne tag
wm-rules-exclude-title = Tranne titolo
wm-rules-exclude-xwayland = Tranne Xwayland
wm-rules-focus = Focus
wm-rules-fullscreen = Schermo intero
wm-rules-group = Gruppo
wm-rules-idle-inhibit = Inibisci inattività
wm-rules-initial-app-id = ID iniziale dell'app
wm-rules-initial-title = Titolo iniziale
wm-rules-is-captured = È acquisita
wm-rules-is-floating = È fluttuante
wm-rules-is-focused = Ha il focus
wm-rules-is-fullscreen = È a schermo intero
wm-rules-is-urgent = È urgente
wm-rules-keep-aspect = Mantieni le proporzioni
wm-rules-max-size = Dimensione massima
wm-rules-min-size = Dimensione minima
wm-rules-maximized = Massimizzata
wm-rules-output = Uscita
wm-rules-pin = Fissa
wm-rules-position = Posizione
wm-rules-rounding = Raggio degli angoli
wm-rules-scroll-factor = Fattore di scorrimento
wm-rules-shadow = Ombra
wm-rules-suppress = Richieste ignorate
wm-rules-workspace = Spazio di lavoro
wm-rules-xwayland = Xwayland
wm-section-focus = Focus
    .description = Cosa ottiene una finestra che chiede il focus
wm-focus-on-activate = All'attivazione
    .description = Dà il focus a una finestra che lo chiede subito dopo che hai usato l'app che l'ha richiesto, altrimenti la segna come urgente; oppure la segna sempre come urgente
wm-rules-on-activate = All'attivazione
    .description = Cosa ottiene questa finestra quando chiede il focus: il focus, lo stato urgente o niente
wm-section-app-shortcuts = Scorciatoie delle app
    .description = Scorciatoie registrate dalle app tramite il portale, ognuna attivata tramite un'associazione globale
wm-app-shortcuts = Scorciatoie registrate
    .description = Registra una combinazione per associarla, Backspace per rimuoverla
wm-app-shortcuts-none = Nessuna app ha ancora registrato una scorciatoia
wm-app-shortcuts-hotkey = L'app gestisce questo tasto da sé, impostato nella sua finestra di dialogo del portale
wm-rules-content = Contenuto
    .description = Cosa mostra la finestra (foto, video o gioco), al di sopra di quanto dichiara essa stessa
wm-app-shortcuts-reload = Cerca di nuovo
wm-section-window-behavior = Comportamento delle finestre
    .description = Come si comportano le finestre nuove, mobili e a schermo intero
wm-section-titlebars = Barre del titolo
    .description = Barre con pulsanti sopra le finestre mobili
wm-section-mica = Mica
    .description = Una sfocatura intensa dello sfondo dietro le finestre traslucide
wm-section-hot-corners = Angoli attivi
    .description = Cosa succede spingendo il puntatore in un angolo dello schermo
wm-section-layout = Disposizione
    .description = Come un nuovo spazio di lavoro dispone le sue finestre
wm-section-cursor = Puntatore
    .description = Quando il puntatore si nasconde e come trovarlo
wm-focus-follows-mouse = Il focus segue il puntatore
    .description = Passare sopra una finestra le dà il focus; morbido attende che il puntatore entri in un'altra finestra
wm-focus-raise-on-hover = Alza al passaggio
    .description = Il focus al passaggio porta in primo piano anche una finestra mobile
wm-focus-warp-pointer = Sposta il puntatore con il focus
    .description = Centra il puntatore sulla finestra che una scorciatoia mette a fuoco, o solo al cambio di spazio di lavoro
wm-focus-on-close = Focus dopo la chiusura
    .description = Quale finestra riceve il focus quando si chiude quella attiva
wm-fullscreen-new-window = Nuova finestra su schermo intero
    .description = Se una finestra aperta sopra una a schermo intero o ingrandita resta dietro, la sostituisce o la chiude
wm-hold-opening = Attendi le vicine
    .description = Una nuova finestra attende fino a 100 ms che le vicine ridimensionate si ridisegnino
wm-floating-step = Passo di spostamento mobile
    .description = Pixel di cui una scorciatoia sposta una finestra mobile
wm-section-swallow = Assorbimento nel terminale
    .description = Un terminale si nasconde dietro la finestra che apre finché questa non si chiude
wm-swallow-terminals = Terminali che assorbono
    .description = Una finestra avviata da uno di questi terminali ne prende il posto finché non si chiude
wm-swallow-except-title = Titoli che non assorbono mai
    .description = Un'espressione regolare sul titolo della nuova finestra; le finestre corrispondenti si aprono accanto al terminale
wm-swallow-terminals-none = Nessun terminale assorbe le finestre che avvia
wm-swallow-terminals-add = Aggiungi terminale
wm-swallow-terminals-written = Un modello scritto a mano, che un elenco di terminali non può esprimere
wm-swallow-terminals-replace = Sostituisci con un elenco
wm-titlebars-floating = Mostra le barre del titolo
    .description = Barre con pulsanti sopra le finestre mobili
wm-titlebars-height = Altezza della barra
    .description = Pixel, presi dalla finestra mobile
wm-titlebars-buttons = Pulsanti
    .description = Quali pulsanti mostra la barra, in ordine
wm-titlebars-side = Lato dei pulsanti
    .description = A quale estremità della barra stanno i pulsanti
wm-titlebars-double-click = Doppio clic
    .description = Cosa fa un doppio clic sulla barra
wm-titlebars-active-background = Sfondo barra del titolo, attiva
    .description = Colore: barra del titolo, attiva
wm-titlebars-active-text = Testo barra del titolo, attiva
    .description = Colore del testo: barra del titolo, attiva
wm-titlebars-inactive-background = Sfondo barra del titolo, inattiva
    .description = Colore: barra del titolo, inattiva
wm-titlebars-inactive-text = Testo barra del titolo, inattiva
    .description = Colore del testo: barra del titolo, inattiva
wm-titlebars-urgent-background = Sfondo barra del titolo, urgente
    .description = Colore: barra del titolo, urgente
wm-titlebars-urgent-text = Testo barra del titolo, urgente
    .description = Colore del testo: barra del titolo, urgente
wm-tabs-active-background = Sfondo scheda, attiva
    .description = Colore: scheda, attiva
wm-tabs-active-text = Testo scheda, attiva
    .description = Colore del testo: scheda, attiva
wm-tabs-inactive-background = Sfondo scheda, inattiva
    .description = Colore: scheda, inattiva
wm-tabs-inactive-text = Testo scheda, inattiva
    .description = Colore del testo: scheda, inattiva
wm-tabs-urgent-background = Sfondo scheda, urgente
    .description = Colore: scheda, urgente
wm-tabs-urgent-text = Testo scheda, urgente
    .description = Colore del testo: scheda, urgente
wm-titlebars-lights-close = Colore di Chiudi
    .description = Colore del punto del pulsante Chiudi
wm-titlebars-lights-minimize = Colore di Riduci
    .description = Colore del punto del pulsante Riduci
wm-titlebars-lights-maximize = Colore di Ingrandisci
    .description = Colore del punto del pulsante Ingrandisci
wm-decoration-blur-live-tiled = Sfocatura dal vivo per finestre affiancate
    .description = Sfoca ciò che sta dietro ogni finestra affiancata invece della sfocatura condivisa
wm-decoration-blur-mica-enabled = Mica
    .description = Disegna le finestre traslucide su una sfocatura intensa dello sfondo
wm-decoration-blur-mica-size = Dimensione sfocatura mica
    .description = Distanza tra i campioni di sfocatura, in pixel
wm-decoration-blur-mica-passes = Passaggi sfocatura mica
    .description = Ogni passaggio raddoppia la portata
wm-decoration-blur-mica-tint = Tinta mica
    .description = Colore sopra lo sfondo mica
wm-drag-preview-snap-windows = Aggancia alle finestre
    .description = Le finestre mobili si agganciano anche accanto o in linea con altre
wm-drag-preview-drag-switch-width = Zona di cambio al bordo
    .description = Pixel al bordo senza vicini dove tenere un trascinamento cambia spazio
wm-drag-preview-drag-switch-frames = Ritardo di cambio al bordo
    .description = Fotogrammi in cui il trascinamento deve restare al bordo
wm-hot-corners-corners = Angoli
    .description = Scegli un angolo, poi cosa fa spingere il puntatore al suo interno
wm-hot-corners-top-left = In alto a sinistra
    .description = Azione quando il puntatore viene spinto in questo angolo
wm-hot-corners-top-right = In alto a destra
    .description = Azione quando il puntatore viene spinto in questo angolo
wm-hot-corners-bottom-left = In basso a sinistra
    .description = Azione quando il puntatore viene spinto in questo angolo
wm-hot-corners-bottom-right = In basso a destra
    .description = Azione quando il puntatore viene spinto in questo angolo
wm-hot-corners-corner-action = Azione
    .description = Cosa fa l'angolo
wm-hot-corners-corner-modifiers = Tasti tenuti
    .description = Tasti da tenere premuti
wm-hot-corners-pressure = Spinta nell'angolo
    .description = Quanto deve spingere il puntatore nell'angolo
wm-hot-corners-fullscreen = Angoli a schermo intero
    .description = Gli angoli agiscono anche con una finestra a schermo intero attiva
wm-showdesktop-peek = Bordo mostra desktop
    .description = Percentuale di ogni finestra lasciata visibile mostrando il desktop
wm-switcher-open-delay = Ritardo del selettore finestre
    .description = Millisecondi di pressione dei tasti prima che appaia il selettore
wm-layout = Disposizione nuovi spazi
    .description = Divisioni e gruppi di schede, o una striscia di colonne scorrevole
wm-scroll-center = Centra la colonna attiva
    .description = Quando la striscia scorre la colonna attiva al centro
wm-input-keyboard-numlock = Bloc Num attivo
    .description = Attiva Bloc Num all'avvio e dopo ogni ricaricamento
wm-input-keyboard-sticky-keys = Tasti permanenti
    .description = Un modificatore premuto da solo vale per il tasto successivo; due volte si blocca
wm-input-keyboard-slow-keys = Tasti lenti
    .description = Millisecondi di pressione perché un tasto valga; 0 disattiva
wm-input-pointer-tap = Tocca per fare clic
    .description = Un tocco fa clic, senza premere
wm-input-pointer-drag-lock = Blocco del trascinamento
    .description = Il trascinamento continua dopo aver sollevato il dito
wm-input-pointer-disable-while-typing = Spento durante la digitazione
    .description = Ignora il dispositivo mentre si digita
wm-input-pointer-disabled-on-external-mouse = Spento con un mouse collegato
    .description = Spegni il dispositivo mentre è collegato un altro puntatore
wm-input-pointer-left-handed = Per mancini
    .description = Scambia i pulsanti principale e secondario
wm-input-pointer-click-method = Metodo di clic
    .description = Come viene letto un clic: aree pulsante o numero di dita
wm-input-pointer-scroll-method = Metodo di scorrimento
    .description = Due dita, il bordo o tenendo un pulsante
wm-input-pointer-scroll-button-lock = Il pulsante di scorrimento alterna
    .description = Un clic sul pulsante di scorrimento lo attiva e disattiva
wm-input-pointer-tap-button-map = Pulsanti al tocco
    .description = Quali pulsanti premono i tocchi con uno, due e tre dita
wm-input-touchpad-disabled-on-external-mouse = Spento con un mouse collegato
    .description = Spegni il dispositivo mentre è collegato un altro puntatore
wm-input-touchpad-left-handed = Per mancini
    .description = Scambia i pulsanti principale e secondario
wm-input-touchpad-scroll-button-lock = Il pulsante di scorrimento alterna
    .description = Un clic sul pulsante di scorrimento lo attiva e disattiva
wm-input-touchpad-tap-button-map = Pulsanti al tocco
    .description = Quali pulsanti premono i tocchi con uno, due e tre dita
wm-input-trackpoint-tap = Tocca per fare clic
    .description = Un tocco fa clic, senza premere
wm-input-trackpoint-drag-lock = Blocco del trascinamento
    .description = Il trascinamento continua dopo aver sollevato il dito
wm-input-trackpoint-disable-while-typing = Spento durante la digitazione
    .description = Ignora il dispositivo mentre si digita
wm-input-trackpoint-disabled-on-external-mouse = Spento con un mouse collegato
    .description = Spegni il dispositivo mentre è collegato un altro puntatore
wm-input-trackpoint-left-handed = Per mancini
    .description = Scambia i pulsanti principale e secondario
wm-input-trackpoint-click-method = Metodo di clic
    .description = Come viene letto un clic: aree pulsante o numero di dita
wm-input-trackpoint-scroll-button-lock = Il pulsante di scorrimento alterna
    .description = Un clic sul pulsante di scorrimento lo attiva e disattiva
wm-input-trackpoint-tap-button-map = Pulsanti al tocco
    .description = Quali pulsanti premono i tocchi con uno, due e tre dita
wm-gestures-edge-width = Zona tattile del bordo
    .description = Pixel dal bordo dove inizia uno scorrimento
wm-cursor-hide-while-typing = Nascondi durante la digitazione
    .description = Nascondi il puntatore mentre si digita in una finestra
wm-cursor-hide-after = Nascondi a riposo
    .description = Millisecondi senza movimento prima di nasconderlo; 0 mai
wm-cursor-shake-to-find = Scuoti per trovare
    .description = Scuotere il mouse ingrandisce brevemente il puntatore
wm-hidden-frame-rate = Frequenza finestre nascoste
    .description = Fotogrammi al secondo per finestre coperte o fuori schermo
wm-power-saver-frame-rate = Frequenza in risparmio energetico
    .description = Frequenza massima con il profilo di risparmio; 0 disattiva
wm-client-hotkeys = Scorciatoie globali delle app
    .description = Consenti alle app scorciatoie globali che la finestra attiva non vede
wm-side-all = Tutti i lati
wm-side-same = Uguale su tutti i lati
wm-side-window = Finestra
enum-wm-accel-profile-adaptive = Adattiva
enum-wm-accel-profile-flat = Piatta
enum-wm-buttons-close = Chiudi
enum-wm-buttons-maximize = Massimizza
enum-wm-buttons-minimize = Riduci a icona
enum-wm-center-always = Sempre
enum-wm-center-never = Mai
enum-wm-center-on-overflow = Quando trabocca
enum-wm-click-method-button_areas = Aree dei pulsanti
enum-wm-click-method-clickfinger = Numero di dita
enum-wm-click-method-none = Nessuno
enum-wm-double-click-float = Rendi flottante
enum-wm-double-click-maximize = Massimizza
enum-wm-follows-mouse-always = Sempre sotto il puntatore
enum-wm-follows-mouse-loose = Quando il puntatore entra in un’altra finestra
enum-wm-follows-mouse-off = Clic per il focus
enum-wm-layout-scroll = Colonne scorrevoli
enum-wm-layout-tree = Albero a tasselli
enum-wm-lid-action-auto = Automatico
enum-wm-lid-action-hibernate = Iberna
enum-wm-lid-action-hybrid-sleep = Sospensione ibrida
enum-wm-lid-action-ignore = Non fare nulla
enum-wm-lid-action-suspend = Sospendi
enum-wm-lid-action-suspend-then-hibernate = Sospendi, poi iberna
enum-wm-new-window-behind = Si apre dietro
enum-wm-new-window-exit = Esce dallo schermo intero
enum-wm-new-window-take_over = Prende lo schermo intero
enum-wm-on-activate-focus = Darle il focus
enum-wm-on-activate-urgent = Segnarla come urgente
enum-wm-on-close-next = La finestra successiva
enum-wm-on-close-under_pointer = La finestra sotto il puntatore
enum-wm-scroll-method-edge = Bordo
enum-wm-scroll-method-none = Nessuno
enum-wm-scroll-method-on_button_down = Con un pulsante premuto
enum-wm-scroll-method-two_finger = Due dita
enum-wm-side-left = Sinistra
enum-wm-side-right = Destra
enum-wm-tap-button-map-lmr = Sinistro, centrale, destro
enum-wm-tap-button-map-lrm = Sinistro, destro, centrale
enum-wm-warp-pointer-off = Mai
enum-wm-warp-pointer-on_keyboard_focus = Quando la tastiera sposta il focus
enum-wm-warp-pointer-on_workspace_switch = Al cambio di area di lavoro
enum-wm-flags-locked = Funziona anche da bloccato
enum-wm-flags-repeating = Si ripete tenendo premuto
enum-wm-flags-release = Al rilascio
enum-wm-flags-pass = Arriva anche alla finestra
enum-wm-flags-inhibit_proof = Funziona anche se le scorciatoie sono inibite
enum-wm-scope-workspace = Questa area di lavoro
enum-wm-scope-output = Questo schermo
enum-wm-scope-app = Questa applicazione
enum-wm-toggle-fullscreen = Solo a schermo intero
enum-wm-vrr-off = Disattivo
enum-wm-vrr-on = Sempre
enum-wm-vrr-fullscreen = Solo a schermo intero
enum-wm-transform-normal = Normale
enum-wm-transform-90 = Ruotato di 90°
enum-wm-transform-180 = Ruotato di 180°
enum-wm-transform-270 = Ruotato di 270°
enum-wm-transform-flipped = Capovolto
enum-wm-transform-flipped-90 = Capovolto, ruotato di 90°
enum-wm-transform-flipped-180 = Capovolto, ruotato di 180°
enum-wm-transform-flipped-270 = Capovolto, ruotato di 270°
enum-wm-content-none = Nessuno
enum-wm-content-game = Gioco
enum-wm-content-photo = Foto
enum-wm-content-video = Video
enum-wm-on-activate-ignore = Ignorarla
settings-nav-wm-keyboard = Tastiera
settings-page-wm-keyboard = Tastiera
settings-nav-wm-pointer = Mouse e touchpad
settings-page-wm-pointer = Mouse e touchpad
wm-section-typing = Digitazione
    .description = Quanto velocemente si ripetono i tasti tenuti premuti e Bloc Num all’avvio
wm-section-keyboard-accessibility = Accessibilità
    .description = Tasti che restano premuti o attendono, per digitare un tasto alla volta
wm-section-keyboard-model = Modello di tastiera
    .description = Il modello e le regole di tastiera per cui sono costruiti i layout
wm-palette-new-name = Nome del nuovo colore
wm-palette-add = Aggiungi colore
wm-palette-reset = Torna al colore di lys-wm
wm-palette-taken = Esiste già un colore chiamato { $name }
wm-palette-add-failed = Il colore { $name } non è stato aggiunto
wm-palette-reset-failed = Il colore { $name } non è stato ripristinato

## What a bind does, by lys-wm action: $first is the first argument, or default without any; $args all of them; $rest all after the first
wm-action-focus = { $first ->
        [left] Metti a fuoco la finestra a sinistra
        [right] Metti a fuoco la finestra a destra
        [up] Metti a fuoco la finestra sopra
        [down] Metti a fuoco la finestra sotto
       *[other] Metti a fuoco { $args }
    }
wm-action-move = { $first ->
        [left] Sposta la finestra a sinistra
        [right] Sposta la finestra a destra
        [up] Sposta la finestra in alto
        [down] Sposta la finestra in basso
       *[other] Sposta la finestra { $args }
    }
wm-action-workspace = { $first ->
        [previous] Passa allo spazio di lavoro precedente
        [empty] Passa a uno spazio di lavoro vuoto
       *[other] Passa allo spazio di lavoro { $args }
    }
wm-action-movetoworkspace = { $first ->
        [previous] Sposta la finestra nello spazio di lavoro precedente
        [empty] Sposta la finestra in uno spazio di lavoro vuoto
       *[other] Sposta la finestra nello spazio di lavoro { $args }
    }
wm-action-focusoutput = { $first ->
        [left] Metti a fuoco lo schermo a sinistra
        [right] Metti a fuoco lo schermo a destra
        [up] Metti a fuoco lo schermo sopra
        [down] Metti a fuoco lo schermo sotto
       *[other] Metti a fuoco lo schermo { $args }
    }
wm-action-movetooutput = { $first ->
        [left] Sposta la finestra sullo schermo a sinistra
        [right] Sposta la finestra sullo schermo a destra
        [up] Sposta la finestra sullo schermo sopra
        [down] Sposta la finestra sullo schermo sotto
       *[other] Sposta la finestra sullo schermo { $args }
    }
wm-action-moveworkspacetooutput = { $first ->
        [left] Sposta lo spazio di lavoro sullo schermo a sinistra
        [right] Sposta lo spazio di lavoro sullo schermo a destra
        [up] Sposta lo spazio di lavoro sullo schermo sopra
        [down] Sposta lo spazio di lavoro sullo schermo sotto
       *[other] Sposta lo spazio di lavoro sullo schermo { $args }
    }
wm-action-swapworkspaces = Scambia gli spazi di lavoro degli schermi { $args }
wm-action-scratchpad = { $first ->
        [hide] Nascondi lo scratchpad
       *[other] Mostra o nascondi lo scratchpad { $rest }
    }
wm-action-movetoscratchpad = Sposta la finestra nello scratchpad { $args }
wm-action-kill = Chiudi la finestra
wm-action-focuswindow = Metti a fuoco la finestra { $args }
wm-action-minimize = { $first ->
        [default] Riduci la finestra
       *[other] Riduci la finestra { $args }
    }
wm-action-restore = { $first ->
        [default] Ripristina l’ultima finestra ridotta
       *[other] Ripristina la finestra { $args }
    }
wm-action-overview = { $first ->
        [show] Mostra la panoramica
        [hide] Nascondi la panoramica
       *[other] Mostra o nascondi la panoramica
    }
wm-action-closewindow = Chiudi la finestra { $args }
wm-action-dpms = { $first ->
        [on] Accendi gli schermi
        [off] Spegni gli schermi
       *[other] Accendi o spegni gli schermi
    }
wm-action-reload = Ricarica la configurazione
wm-action-dismissconfignotice = Chiudi l’avviso di configurazione
wm-action-exit = Termina la sessione
wm-action-nextlayout = Passa alla disposizione di tastiera successiva
wm-action-stickykeys = { $first ->
        [on] Attiva i tasti permanenti
        [off] Disattiva i tasti permanenti
       *[other] Tasti permanenti { $args }
    }
wm-action-slowkeys = Imposta i tasti lenti a { $args } ms
wm-action-global = Scorciatoia dell’app { $args }
wm-action-exec = Esegui { $args }
wm-action-launch = Avvia { $app }
wm-action-makegroup = { $first ->
        [h] Raggruppa la finestra affiancata
        [v] Raggruppa la finestra in pila
        [tab] Raggruppa la finestra in schede
       *[other] Raggruppa la finestra { $args }
    }
wm-action-setlayout = { $first ->
        [tree] Usa la disposizione ad affiancamento
        [scroll] Usa la disposizione a scorrimento
       *[other] Imposta la disposizione { $args }
    }
wm-action-columnwidth = Imposta la larghezza della colonna { $args }
wm-action-consume = { $first ->
        [left] Sposta la finestra nella colonna a sinistra
        [right] Sposta la finestra nella colonna a destra
       *[other] Sposta la finestra nella colonna { $args }
    }
wm-action-expel = Sposta la finestra in una colonna a sé
wm-action-center = Centra la colonna con focus
wm-action-columndisplay = { $first ->
        [stack] Impila le finestre della colonna
        [tab] Mostra la colonna a schede
       *[other] Mostra la colonna come { $args }
    }
wm-action-changegroup = { $first ->
        [h] Disponi il gruppo affiancato
        [v] Disponi il gruppo in pila
        [tab] Disponi il gruppo a schede
        [toggletab] Attiva o disattiva le schede nel gruppo
        [opposite] Ruota il gruppo nell’altro verso
       *[other] Cambia il gruppo { $args }
    }
wm-action-focustab = { $first ->
        [prev] Metti a fuoco la scheda precedente
        [next] Metti a fuoco la scheda successiva
       *[other] Metti a fuoco la scheda { $args }
    }
wm-action-focusparent = Metti a fuoco il gruppo che lo contiene
wm-action-focuschild = Torna a mettere a fuoco dentro il gruppo
wm-action-swap = { $first ->
        [left] Scambia la finestra con quella a sinistra
        [right] Scambia la finestra con quella a destra
        [up] Scambia la finestra con quella sopra
        [down] Scambia la finestra con quella sotto
       *[other] Scambia la finestra { $args }
    }
wm-action-moveout = Sposta la finestra fuori dal suo gruppo
wm-action-equalize = { $first ->
        [all] Rendi uguali tutte le divisioni
       *[other] Rendi uguale il gruppo
    }
wm-action-locktab = { $first ->
        [on] Blocca il gruppo di schede
        [off] Sblocca il gruppo di schede
       *[other] Blocca o sblocca il gruppo di schede
    }
wm-action-submap = { $first ->
        [reset] Esci dalla modalità
       *[other] Entra nella modalità { $args }
    }
wm-action-togglefloating = Rendi la finestra mobile o affiancata
wm-action-place = { $first ->
        [left] Posiziona la finestra sulla metà sinistra
        [right] Posiziona la finestra sulla metà destra
        [top] Posiziona la finestra sulla metà superiore
        [bottom] Posiziona la finestra sulla metà inferiore
        [topleft] Posiziona la finestra nel quarto in alto a sinistra
        [topright] Posiziona la finestra nel quarto in alto a destra
        [bottomleft] Posiziona la finestra nel quarto in basso a sinistra
        [bottomright] Posiziona la finestra nel quarto in basso a destra
        [fill] Fai riempire lo schermo alla finestra
        [center] Centra la finestra
        [restore] Riporta la finestra dov’era
       *[other] Posiziona la finestra { $args }
    }
wm-action-focusfloating = { $first ->
        [on] Metti a fuoco le finestre mobili
        [off] Metti a fuoco le finestre affiancate
       *[other] Alterna il focus tra finestre mobili e affiancate
    }
wm-action-fullscreen = { $first ->
        [on] Metti la finestra a schermo intero
        [off] Esci dallo schermo intero
       *[other] Attiva o disattiva lo schermo intero
    }
wm-action-setprop = Imposta la proprietà della finestra { $args }
wm-action-showdesktop = { $first ->
        [on] Mostra la scrivania
        [off] Riporta le finestre
       *[other] Mostra o nascondi la scrivania
    }
wm-action-swallow = { $first ->
        [on] Nascondi il terminale nella finestra
        [off] Mostra il terminale nascosto
       *[other] Mostra o nascondi il terminale inglobato
    }
wm-action-pin = { $first ->
        [on] Fissa la finestra
        [off] Sblocca la finestra
       *[other] Fissa o sblocca la finestra
    }
wm-action-maximize = { $first ->
        [on] Ingrandisci la finestra
        [off] Ripristina la dimensione della finestra
       *[other] Attiva o disattiva l’ingrandimento
    }
wm-action-resizeactive = Ridimensiona la finestra di { $args }
wm-action-movewindow = Trascina la finestra
wm-action-resizewindow = Ridimensiona la finestra trascinando
wm-action-zoom = { $first ->
        [in] Ingrandisci lo zoom
        [out] Riduci lo zoom
        [reset] Reimposta lo zoom
       *[other] Imposta lo zoom a { $rest }
    }
wm-action-debug = Debug { $args }
wm-action-casttarget = { $first ->
        [window] Condividi una finestra
        [output] Condividi uno schermo
        [clear] Interrompi la condivisione
       *[other] Condividi { $args }
    }
wm-action-focuslast = Metti a fuoco la finestra precedente
wm-action-focusurgent = Metti a fuoco la finestra che richiede attenzione
wm-action-cycle = { $first ->
        [next] Passa alla finestra successiva
        [prev] Passa alla finestra precedente
       *[other] Scorri le finestre { $args }
    }
wm-action-switcher = { $first ->
        [next] Cambia con la finestra successiva
        [prev] Cambia con la finestra precedente
       *[other] Cambia finestra { $args }
    }
wm-binds-unknown-action = Azione sconosciuta
    .description = lys-wm non ha alcuna azione con questo nome; la scorciatoia è mostrata come scritta
wm-binds-category-windows = Finestre
wm-binds-category-workspaces = Spazi di lavoro
wm-binds-category-layout = Disposizione
wm-binds-category-apps = App
wm-binds-category-screenshots = Schermate
wm-binds-category-media = Multimedia
wm-binds-category-system = Sistema
wm-binds-category-custom = Personalizzate
wm-binds-search = Cerca scorciatoie, azioni o tasti

## The arguments of a lys-wm action, by the form lys-wm describes
wm-action-pick = Scegli un’azione
wm-action-args-outside = Non è una forma di questa azione
    .description = lys-wm descrive diversamente gli argomenti di questa azione; modificali come testo
enum-wm-arg-default = Predefinito
enum-wm-arg-all = Tutti
enum-wm-arg-app = App con focus
enum-wm-arg-app_id = ID app
enum-wm-arg-app_id-colon-id = App e scorciatoia
enum-wm-arg-bottom = Metà inferiore
enum-wm-arg-bottomleft = In basso a sinistra
enum-wm-arg-bottomright = In basso a destra
enum-wm-arg-center = Centro
enum-wm-arg-clear = Nessuno
enum-wm-arg-command = Comando
enum-wm-arg-down = Giù
enum-wm-arg-dx = Variazione larghezza (px)
enum-wm-arg-dy = Variazione altezza (px)
enum-wm-arg-e-minus-n = Apri spazio prima, qualsiasi schermo
enum-wm-arg-e-plus-n = Apri spazio dopo, qualsiasi schermo
enum-wm-arg-empty = Primo vuoto
enum-wm-arg-ephemeral = Sciogli con una finestra
enum-wm-arg-factor = Fattore
enum-wm-arg-fake = Solo nel suo riquadro
enum-wm-arg-fill = Riempi
enum-wm-arg-follow = Segui la finestra
enum-wm-arg-h = Affiancate
enum-wm-arg-hide = Nascondi
enum-wm-arg-id = ID finestra
enum-wm-arg-in = Dentro
enum-wm-arg-key = Proprietà
enum-wm-arg-left = Sinistra
enum-wm-arg-m-minus-n = Apri spazio prima, questo schermo
enum-wm-arg-m-plus-n = Apri spazio dopo, questo schermo
enum-wm-arg-minus-n-percent = Più stretta in percentuale
enum-wm-arg-minus-n-px = Più stretta in pixel
enum-wm-arg-ms = Millisecondi
enum-wm-arg-n = Numero
enum-wm-arg-n-percent = Percentuale dello schermo
enum-wm-arg-n-px = Pixel
enum-wm-arg-name = Nome
enum-wm-arg-next = Successivo
enum-wm-arg-off = Disattivo
enum-wm-arg-on = Attivo
enum-wm-arg-opposite = Nell’altro verso
enum-wm-arg-out = Fuori
enum-wm-arg-output = Schermo
enum-wm-arg-output-name = Schermo
enum-wm-arg-plus-n-percent = Più larga in percentuale
enum-wm-arg-plus-n-px = Più larga in pixel
enum-wm-arg-preset-next = Preimpostazione successiva
enum-wm-arg-preset-prev = Preimpostazione precedente
enum-wm-arg-prev = Precedente
enum-wm-arg-previous = Precedente
enum-wm-arg-reset = Reimposta
enum-wm-arg-restore = Dov’era
enum-wm-arg-right = Destra
enum-wm-arg-scroll = A scorrimento
enum-wm-arg-set = Imposta
enum-wm-arg-show = Mostra
enum-wm-arg-stack = In pila
enum-wm-arg-tab = Schede
enum-wm-arg-toggle = Alterna
enum-wm-arg-toggle-name = Alterna
enum-wm-arg-toggletab = Alterna schede
enum-wm-arg-top = Metà superiore
enum-wm-arg-topleft = In alto a sinistra
enum-wm-arg-topright = In alto a destra
enum-wm-arg-tree = Affiancamento
enum-wm-arg-up = Su
enum-wm-arg-v = In pila
enum-wm-arg-value = Valore
enum-wm-arg-window = Finestra
enum-wm-arg-window-id = Finestra
enum-wm-arg-workspace = Spazio di lavoro
enum-wm-arg-wrap = Ricomincia da capo
wm-binds-clash-mode = { $label } nella modalità { $mode }
wm-binds-reset = Torna alla scorciatoia di lys-wm
wm-mode = Modalità { $mode }
wm-mode-enter = Scorciatoia che entra nella modalità { $mode }
wm-mode-rename = Rinomina la modalità
wm-mode-rename-confirm = Rinomina
wm-mode-delete-button = Elimina la modalità
wm-mode-delete-title = Eliminare la modalità { $mode }?
wm-mode-delete-description = Vengono rimosse anche le sue scorciatoie e quelle che vi entrano.
wm-mode-delete-confirm = Elimina
wm-mode-add = Aggiungi modalità
wm-mode-add-confirm = Aggiungi
wm-mode-name = Nome
wm-mode-name-placeholder = Nome, come resize
wm-mode-chord = Scorciatoia che entra nella modalità
wm-mode-name-missing = Una modalità ha bisogno di un nome
wm-mode-name-reset = { $name } è il modo in cui una scorciatoia esce da una modalità, quindi non può darle il nome
wm-mode-name-taken = Esiste già una modalità chiamata { $name }
wm-mode-chord-missing = Registra la scorciatoia che entra nella modalità
wm-mode-refused = lys-wm non accetta questa modifica
wm-mode-not-saved = La modifica non è stata salvata
wm-mode-delete-failed = Impossibile eliminare la modalità
wm-mode-unreachable = Nessuna scorciatoia entra in questa modalità; registrane una accanto al suo nome
wm-section-refraction = Rifrazione
    .description = Vetro sfocato che piega ciò che ha dietro vicino ai bordi
wm-section-specular = Riflesso del bordo
    .description = Un riflesso lungo il bordo del vetro sfocato, dal lato rivolto al puntatore
wm-decoration-blur-refraction = Rifrazione
    .description = Piega lo sfondo vicino ai bordi delle finestre sfocate
wm-decoration-blur-refraction-bezel = Bordo di piegatura
    .description = Fin dove, dal bordo verso l'interno, si piega lo sfondo, in pixel
wm-decoration-blur-refraction-amount = Intensità della rifrazione
    .description = Fin dove il vetro guarda oltre il bordo, in pixel
wm-decoration-blur-specular = Riflesso del bordo
    .description = Illumina il bordo delle finestre sfocate
wm-decoration-blur-specular-strength = Luminosità del riflesso
    .description = Quanto brilla il bordo
wm-decoration-blur-specular-width = Larghezza del riflesso
    .description = In pixel
wm-scroll-presets = Larghezze delle colonne
    .description = Le larghezze che la scorciatoia per la larghezza della colonna percorre nel layout a scorrimento, in quest'ordine
    .keywords = preimpostazioni, columnwidth, scorrimento
wm-scroll-presets-share = Parte dello schermo
wm-scroll-presets-pixels = Pixel
wm-scroll-presets-add = Aggiungi larghezza
wm-scroll-presets-add-width = Aggiungi una colonna larga { $width }
wm-scroll-presets-remove = Rimuovi questa larghezza
wm-section-touch-gestures = Gesti del touchscreen
    .description = Gli scorrimenti su un touchscreen e cosa fanno
wm-gestures-touch = Scorrimenti touch
    .description = Scorrere dal bordo, o con tre o più dita in qualsiasi punto dello schermo
wm-gestures-touch-add = Aggiungi scorrimento touch
wm-gestures-touch-start = Inizia
wm-gestures-touch-does = Fa
wm-gestures-touch-fingers-anywhere = Dita ovunque
wm-gestures-touch-fingers = Dita
wm-gestures-touch-direction = Direzione
wm-gestures-touch-run = Esegui un'azione
wm-gestures-touch-follow = Segui le dita
wm-gestures-touch-edge-title = { $edge ->
        [top] Scorri dal bordo superiore
        [bottom] Scorri dal bordo inferiore
        [left] Scorri dal bordo sinistro
        [right] Scorri dal bordo destro
       *[other] Scorri da un bordo
    }
wm-gestures-touch-fingers-title = { $direction ->
        [left] Scorri con { $fingers } dita a sinistra
        [right] Scorri con { $fingers } dita a destra
        [up] Scorri con { $fingers } dita in alto
        [down] Scorri con { $fingers } dita in basso
        [horizontal] Scorri con { $fingers } dita di lato
        [vertical] Scorri con { $fingers } dita in alto o in basso
        [any] Scorri con { $fingers } dita in ogni direzione
       *[other] Scorri con { $fingers } dita
    }
enum-wm-edge-top = Bordo superiore
enum-wm-edge-bottom = Bordo inferiore
enum-wm-edge-left = Bordo sinistro
enum-wm-edge-right = Bordo destro
enum-wm-direction-left = Sinistra
enum-wm-direction-right = Destra
enum-wm-direction-up = Su
enum-wm-direction-down = Giù
enum-wm-direction-horizontal = Di lato
enum-wm-direction-vertical = Su o giù
enum-wm-direction-any = Ogni direzione
enum-wm-drive-workspaces = Spazi di lavoro
enum-wm-drive-overview = Panoramica
enum-wm-drive-view = Scorri le colonne
enum-wm-drive-move = Sposta la finestra
enum-wm-drive-resize = Ridimensiona la finestra
wm-input-pointer-scroll-button = Pulsante di scorrimento
    .description = Il pulsante tenuto premuto per scorrere muovendo il dispositivo
wm-input-touchpad-scroll-button = Pulsante di scorrimento
    .description = Il pulsante tenuto premuto per scorrere muovendo il dispositivo
wm-input-trackpoint-scroll-button = Pulsante di scorrimento
    .description = Il pulsante tenuto premuto per scorrere muovendo il dispositivo
wm-button-code = Codice del pulsante
    .description = Un altro pulsante, tramite il suo codice evdev (BTN_SIDE è 275)
enum-wm-scroll-button-back = Indietro
enum-wm-scroll-button-extra = Extra
enum-wm-scroll-button-forward = Avanti
enum-wm-scroll-button-left = Sinistro
enum-wm-scroll-button-middle = Centrale
enum-wm-scroll-button-right = Destro
enum-wm-scroll-button-side = Laterale
enum-wm-scroll-button-task = Attività
wm-preview-tabs = Anteprima della barra delle schede
wm-preview-tab-active = Attiva
wm-preview-tab-inactive = Altra finestra
wm-preview-tab-urgent = Chiede attenzione
wm-tabs-font-built-in = Il carattere di lys-wm
wm-tabs-font-unresolved = Nessun file di carattere di { $family } trovato
wm-tabs-font-unreadable = lys-wm non riesce a leggere il file di carattere di { $family }
wm-tabs-font-file-unreadable = Il file del carattere non può essere letto
wm-section-workspaces = Spazi di lavoro
    .description = Regole per singoli spazi di lavoro, o per ogni spazio che corrisponde
wm-section-scratchpads = Finestre rapide
    .description = Finestre con nome mostrate e nascoste sopra le altre con una scorciatoia
wm-workspaces = Regole degli spazi di lavoro
    .description = Uno spazio per numero, o ogni spazio che corrisponde, e come appare e si avvia
wm-workspace-number = Spazio di lavoro { $number }
wm-workspaces-id = Numero
wm-workspaces-match = Ogni spazio
wm-workspaces-name = Nome
wm-workspaces-output = Schermo
wm-workspaces-default = Mostrato per primo sul suo schermo
wm-workspaces-persistent = Mantenuto anche se vuoto
wm-workspaces-on-created-empty = App da aprire quando è creato vuoto
wm-workspaces-layout = Disposizione
wm-workspaces-gaps-inner = Spazio tra le finestre
wm-workspaces-gaps-outer = Spazio ai bordi dello schermo
wm-workspaces-borders-width = Larghezza del bordo
wm-workspaces-rounding = Arrotondamento degli angoli
wm-workspaces-shadow = Ombra
wm-scratchpads = Finestre rapide
    .description = L’app e la dimensione di ogni finestra rapida; una scorciatoia la mostra o nasconde per nome
wm-scratchpads-name = Nome della finestra rapida
wm-scratchpads-exec = App
wm-scratchpads-size = Dimensione
wm-scratchpads-hide-on-workspace-switch = Nascosta al cambio di spazio
wm-size-width = Larghezza %
wm-size-height = Altezza %
enum-wm-match-any = Qualsiasi spazio
enum-wm-match-single_tiled = Con una sola finestra affiancata
enum-wm-match-fullscreen = Con una finestra a schermo intero
wm-section-animation-events = Animazioni del gestore delle finestre
    .description = Come si muove ogni animazione di lys-wm; un valore vuoto segue l’animazione sopra
wm-section-curves = Curve di animazione
    .description = Le tue curve di accelerazione, che le animazioni usano per nome
wm-animations-events = Animazioni
    .description = Se ogni animazione viene riprodotta, la sua curva, la sua durata e il suo stile
wm-animations-curves = Curve
    .description = Una curva disegnata o una molla ciascuna, con un nome per le animazioni
wm-animations-curves-name = Nome della curva
wm-animations-curves-shape = Forma
wm-curve-kind-bezier = Curva
wm-curve-kind-spring = Molla
wm-curve-canvas = Forma della curva; trascina le due maniglie per cambiarla
wm-curve-x1 = Prima maniglia, tempo
wm-curve-y1 = Prima maniglia, avanzamento
wm-curve-x2 = Seconda maniglia, tempo
wm-curve-y2 = Seconda maniglia, avanzamento
wm-curve-stiffness = Rigidità
wm-curve-damping = Smorzamento
wm-curve-mass = Massa
wm-curve-presets = Parti da
wm-accel-curve-none = Il profilo di accelerazione decide come accelera il puntatore
wm-accel-curve-start = Disegna una curva personalizzata
wm-accel-curve-canvas = Velocità del puntatore rispetto a quella del dispositivo; trascina un punto in su o in giù per cambiarlo
wm-accel-curve-step = Velocità del dispositivo tra i punti
wm-accel-curve-add = Aggiungi punto
wm-accel-curve-remove = Rimuovi l'ultimo punto
wm-accel-curve-stop = Usa il profilo di accelerazione
wm-curve-own = Le tue curve
wm-curve-linear = Lineare
wm-curve-ease = Morbida
wm-curve-ease_in = Morbida in entrata
wm-curve-ease_out = Morbida in uscita
wm-curve-ease_in_out = Morbida in entrata e in uscita
wm-curve-spring = Molla
wm-anim-column-enabled = Riprodotta
wm-anim-column-curve = Curva
wm-anim-column-duration = Durata (ms)
wm-anim-column-style = Stile
wm-anim-event-global = Tutto
wm-anim-event-windows = Finestre
wm-anim-event-windows_move = Spostare finestre
wm-anim-event-windows_in = Aprire finestre
wm-anim-event-windows_out = Chiudere finestre
wm-anim-event-windows_minimize = Ridurre finestre
wm-anim-event-snap = Agganciare finestre
wm-anim-event-fade = Dissolvenze
wm-anim-event-fade_in = Comparsa
wm-anim-event-fade_out = Scomparsa
wm-anim-event-fade_switch = Cambi di focus
wm-anim-event-fade_dim = Oscuramento
wm-anim-event-fade_shadow = Ombre
wm-anim-event-fade_snap = Anteprima dell’aggancio
wm-anim-event-workspaces = Cambiare spazio di lavoro
wm-anim-event-layers = Pannelli e sovrapposizioni
wm-anim-event-layers_in = Pannelli che compaiono
wm-anim-event-layers_out = Pannelli che scompaiono
wm-anim-event-borderangle = Sfumatura del bordo
wm-anim-event-zoom = Zoom
wm-anim-event-overview = Panoramica
wm-anim-event-scratchpad = Finestre rapide
wm-anim-style-popin = Comparsa a scatto
wm-anim-style-slide = Scorrimento
wm-anim-style-slidevert = Scorrimento verticale
wm-anim-style-fade = Dissolvenza
wm-anim-style-once = Una volta
wm-anim-style-loop = In ciclo
wm-anim-style-morph = Metamorfosi
wm-anim-style-elastic = Elastico
wm-anim-style-goo = Viscoso
wm-anim-style-bud = Germoglio
wm-anim-style-genie = Genio
wm-app-shortcuts-unreadable = Impossibile leggere le scorciatoie registrate
wm-devices-reading = Richiesta a lys-wm dei dispositivi collegati…
