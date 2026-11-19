settings-launcher-location = Posizione
    .description = Dove appare l'avviatore sullo schermo

settings-launcher-width = Larghezza
    .description = Larghezza della superficie: un moltiplicatore del valore predefinito (1.0) o pixel (ad es. "800px")

settings-launcher-lines = Righe
    .description = Righe di risultati visibili

settings-launcher-monitor = Monitor
    .description = Connettore dell'uscita su cui mostrarlo (vuoto = uscita con il focus)

settings-launcher-modes = Modalità
    .description = Modalità attive, nell'ordine delle schede (drun, run, window, ssh, filebrowser, combi, keys o il nome di uno script)

settings-launcher-cycle = Selezione ciclica
    .description = Ricomincia la selezione ai bordi dell'elenco

settings-launcher-matching = Metodo di corrispondenza
    .description = Come il testo digitato viene confrontato con le voci

settings-launcher-tokenize = Suddividi in parole
    .description = Confronta ogni parola della ricerca in modo indipendente

settings-launcher-negate-char = Carattere di negazione
    .description = Prefisso che esclude le corrispondenze (rofi -matching-negate-char)

settings-launcher-normalize-match = Normalizza la corrispondenza
    .description = Ignora accenti e varianti Unicode durante il confronto

settings-launcher-sort = Ordina i risultati
    .description = Ordina i risultati per qualità della corrispondenza invece che per ordine dell'elenco

settings-launcher-sorting-method = Metodo di ordinamento
    .description = Algoritmo di classificazione usato quando l'ordinamento è attivo

settings-launcher-case = Maiuscole/minuscole
    .description = Come le maiuscole e le minuscole influiscono sulla corrispondenza

settings-launcher-terminal = Terminale
    .description = Emulatore di terminale per le app da terminale (vuoto = rilevamento automatico)

settings-launcher-show-icons = Mostra icone
    .description = Mostra le icone accanto alle voci

settings-launcher-icon-theme = Tema di icone
    .description = Tema di icone alternativo (vuoto = tema di sistema)

settings-launcher-sidebar-mode = Modalità barra laterale
    .description = Mostra le schede delle modalità in fondo all'avviatore

settings-launcher-auto-select = Selezione automatica
    .description = Accetta automaticamente quando resta un solo risultato

settings-launcher-hover-select = Selezione al passaggio
    .description = Seleziona la riga sotto il puntatore del mouse

settings-launcher-fixed-num-lines = Altezza fissa
    .description = Mantiene l'altezza dell'elenco fissa al numero di righe configurato

settings-launcher-display-names = Nomi visualizzati delle modalità
    .description = Nomi visualizzati alternativi per modalità (ad es. drun = "apps")

settings-launcher-scripts = Modalità script
    .description = Modalità script personalizzate: nome → eseguibile (protocollo script di rofi)

settings-launcher-keybindings = Scorciatoie da tastiera
    .description = Scorciatoie alternative: azione → tasti separati da virgole (le azioni non impostate mantengono i valori predefiniti di rofi)

settings-launcher-mouse-bindings = Associazioni del mouse
    .description = Associazioni del mouse alternative: azione (pulsanti me-*, scorrimento ml-*) → pulsanti separati da virgole (ad es. "MouseDPrimary", "ScrollDown")

settings-launcher-font = Carattere
    .description = Carattere dell'avviatore come descrizione Pango (ad es. "Inter 12"); vuoto mantiene il carattere della shell

settings-launcher-styles = Stili predefiniti
    .description = Aspetti con nome selezionabili a ogni invocazione con -style <name>: nome → il CSS applicato per quella sessione

settings-launcher-preview-cmd = Comando di anteprima
    .description = Comando che crea la miniatura di una riga: {"{"}input{"}"} il file, {"{"}output{"}"} l'immagine da scrivere, {"{"}size{"}"} la dimensione in pixel; vuoto usa i generatori di miniature di sistema

settings-launcher-history = Cronologia
    .description = Cronologia degli avvii e classificazione per frecency

settings-launcher-drun = Applicazioni
    .description = Modalità drun (applicazioni)

settings-launcher-run = Comandi
    .description = Modalità run (comandi)

settings-launcher-window = Finestre
    .description = Modalità selettore di finestre

settings-launcher-ssh = SSH
    .description = Modalità host SSH

settings-launcher-filebrowser = Gestore file
    .description = Modalità gestore file

settings-launcher-combi = Combi
    .description = Modalità combinata

settings-launcher-history-enable = Attiva la cronologia
    .description = Registra gli avvii e mette per prime le voci usate di frequente

settings-launcher-history-max-size = Dimensione della cronologia
    .description = Numero massimo di voci ricordate per modalità

settings-launcher-drun-categories = Categorie
    .description = Mostra solo le applicazioni di queste categorie (vuoto = tutte)

settings-launcher-drun-exclude-categories = Categorie escluse
    .description = Nasconde le applicazioni di queste categorie

settings-launcher-drun-match-fields = Campi di ricerca
    .description = Campi della voce desktop cercati durante la digitazione

settings-launcher-drun-display-format = Formato di visualizzazione
    .description = Modello della riga: {"{name}"}, {"{generic}"}, {"{exec}"}, {"{categories}"}, {"{comment}"}; [..] viene mostrato solo se compilato

settings-launcher-drun-show-actions = Mostra azioni
    .description = Mostra le azioni del file desktop come righe aggiuntive

settings-launcher-drun-url-launcher = Apertura URL
    .description = Comando che apre le voci desktop Type=Link (vuoto = l'apertura integrata)

settings-launcher-run-run-command = Comando di esecuzione
    .description = Modello di accettazione semplice ({"{cmd}"})

settings-launcher-run-shell-command = Comando da terminale
    .description = Modello di esecuzione nel terminale ({"{terminal}"}, {"{cmd}"})

settings-launcher-run-list-command = Comando di elenco
    .description = Comando aggiuntivo le cui righe di stdout aggiungono voci

settings-launcher-window-format = Formato di visualizzazione
    .description = Modello della riga: {"{w}"} spazio di lavoro, {"{c}"} classe, {"{t}"} titolo, {"{n}"} nome, {"{r}"} ruolo

settings-launcher-window-match-fields = Campi di ricerca
    .description = Campi della finestra cercati durante la digitazione

settings-launcher-window-hide-active = Nascondi la finestra attiva
    .description = Nasconde dall'elenco la finestra che ha il focus

settings-launcher-window-close-on-delete = Chiudi con Canc
    .description = Maiusc+Canc chiude la finestra selezionata

settings-launcher-ssh-client = Client SSH
    .description = Eseguibile del client SSH

settings-launcher-ssh-command = Comando di connessione
    .description = Modello di connessione ({"{terminal}"}, {"{ssh-client}"}, {"{host}"})

settings-launcher-ssh-parse-hosts = Leggi /etc/hosts
    .description = Include gli host da /etc/hosts

settings-launcher-ssh-parse-known-hosts = Leggi known_hosts
    .description = Include gli host da ~/.ssh/known_hosts

settings-launcher-filebrowser-directory = Cartella iniziale
    .description = Cartella in cui si apre il gestore file (vuoto = home)

settings-launcher-filebrowser-sorting-method = Ordinamento
    .description = Ordine dei file

settings-launcher-filebrowser-directories-first = Cartelle per prime
    .description = Elenca le cartelle prima dei file

settings-launcher-filebrowser-show-hidden = Mostra nascosti
    .description = Mostra i file nascosti

settings-launcher-filebrowser-command = Comando di apertura
    .description = Comando che apre il file scelto (vuoto = l'apertura integrata)

settings-launcher-combi-modes = Modalità combinate
    .description = Modalità unite nell'elenco combinato

settings-launcher-combi-display-format = Formato di visualizzazione
    .description = Modello della riga ({"{mode}"}, {"{text}"})

## LauncherLocation variants
enum-launcher-location-center = Centro
enum-launcher-location-north-west = In alto a sinistra
enum-launcher-location-north = In alto
enum-launcher-location-north-east = In alto a destra
enum-launcher-location-east = Destra
enum-launcher-location-south-east = In basso a destra
enum-launcher-location-south = In basso
enum-launcher-location-south-west = In basso a sinistra
enum-launcher-location-west = Sinistra

## LauncherMatching variants
enum-launcher-matching-normal = Normale
enum-launcher-matching-regex = Espressione regolare
enum-launcher-matching-glob = Glob
enum-launcher-matching-fuzzy = Approssimata
enum-launcher-matching-prefix = Prefisso

## LauncherSorting variants
enum-launcher-sorting-levenshtein = Levenshtein
enum-launcher-sorting-fzf = FZF

## LauncherCase variants
enum-launcher-case-insensitive = Indifferente
enum-launcher-case-smart = Intelligente
enum-launcher-case-sensitive = Distingui

## LauncherDrunField variants
enum-launcher-drun-field-name = Nome
enum-launcher-drun-field-generic = Nome generico
enum-launcher-drun-field-exec = Comando
enum-launcher-drun-field-categories = Categorie
enum-launcher-drun-field-comment = Commento
enum-launcher-drun-field-keywords = Parole chiave

## LauncherWindowField variants
enum-launcher-window-field-title = Titolo
enum-launcher-window-field-class = Classe
enum-launcher-window-field-name = Nome
enum-launcher-window-field-role = Ruolo
enum-launcher-window-field-desktop = Spazio di lavoro

## LauncherFileSort variants
enum-launcher-file-sort-name = Nome
enum-launcher-file-sort-mtime = Modificato
enum-launcher-file-sort-atime = Accesso
enum-launcher-file-sort-ctime = Creato

## Animations
settings-animations-launcher = Avviatore
    .description = Animazione specifica di comparsa e scomparsa per l'avviatore delle applicazioni

settings-launcher-search = Pannello di ricerca
    .description = Il pannello che `lys-shell launcher` apre senza argomenti

settings-launcher-search-providers = Sorgenti
    .description = Cosa cerca l'elenco

settings-launcher-search-web-engine = Nome della ricerca web
    .description = Il nome del motore nella riga di riserva

settings-launcher-search-web-url = URL della ricerca web
    .description = L'URL, con {"{"}query{"}"} dove va inserita la ricerca

settings-launcher-search-file-roots = Cartelle dell'indice file
    .description = Cartelle percorse dall'indice dei file; vuoto indicizza la cartella home

settings-launcher-search-file-exclude = Esclusioni dell'indice file
    .description = Nomi di cartelle che l'indice dei file salta

settings-launcher-search-commands-dir = Cartella dei comandi script
    .description = Vuoto usa launcher/commands nella cartella di configurazione

settings-launcher-search-currency-rates = Tassi di cambio
    .description = Scarica i tassi di riferimento giornalieri della Banca centrale europea, al massimo una volta al giorno, per le risposte sulle valute

settings-launcher-search-previews = Anteprime
    .description = Mostra il riquadro laterale per file, frammenti, finestre e voci degli appunti, nel pannello di ricerca e nelle modalità finestre e appunti

settings-launcher-search-quicklinks = Link rapidi
    .description = Link da aprire per nome o parola chiave

settings-launcher-search-snippets = Frammenti
    .description = Testo da copiare per nome o parola chiave

enum-launcher-search-provider-apps = Applicazioni
enum-launcher-search-provider-windows = Finestre aperte
enum-launcher-search-provider-files = File
enum-launcher-search-provider-calculator = Calcolatrice
enum-launcher-search-provider-system = Comandi di sistema
enum-launcher-search-provider-settings = Pagine delle impostazioni
enum-launcher-search-provider-clipboard = Cronologia degli appunti
enum-launcher-search-provider-emoji = Emoji
enum-launcher-search-provider-characters = Caratteri Unicode
enum-launcher-search-provider-vault = Cassaforte Bitwarden
enum-launcher-search-provider-links = Link rapidi e frammenti
enum-launcher-search-provider-browser-bookmarks = Segnalibri del browser
enum-launcher-search-provider-browser-history = Cronologia del browser
enum-launcher-search-provider-quick-toggles = Interruttori rapidi
enum-launcher-search-provider-media = Controlli multimediali
enum-launcher-search-provider-capture = Schermate e registrazione
enum-launcher-search-provider-processes = Termina processo
enum-launcher-search-provider-scripts = Comandi script
enum-launcher-search-provider-shell = Comandi della shell
enum-launcher-search-provider-web = Ricerca web

## Format placeholders

settings-launcher-drun-display-format-placeholder-name = Nome dell'applicazione

settings-launcher-drun-display-format-placeholder-generic = Nome generico (ad es. Browser web)

settings-launcher-drun-display-format-placeholder-exec = Comando eseguito dalla voce

settings-launcher-drun-display-format-placeholder-categories = Categorie della voce desktop

settings-launcher-drun-display-format-placeholder-comment = Commento della voce desktop

settings-launcher-window-format-placeholder-w = Spazio di lavoro

settings-launcher-window-format-placeholder-c = Classe della finestra

settings-launcher-window-format-placeholder-t = Titolo della finestra

settings-launcher-window-format-placeholder-n = Nome della finestra (la sua classe)

settings-launcher-window-format-placeholder-r = Ruolo della finestra (sempre vuoto)

settings-launcher-combi-display-format-placeholder-mode = Modalità da cui proviene la riga

settings-launcher-combi-display-format-placeholder-text = Il testo della riga

settings-launcher-emoji = Emoji
    .description = Emoji nel pannello di ricerca, nella griglia delle emoji e nella modalità emoji

settings-launcher-emoji-skin-tone = Carnagione
    .description = La carnagione delle emoji di persone e mani; sceglierne una dalle azioni di un'emoji la rende predefinita

## LauncherSkinTone variants
enum-launcher-skin-tone-none = Predefinita (giallo)
enum-launcher-skin-tone-light = Chiara
enum-launcher-skin-tone-medium-light = Medio-chiara
enum-launcher-skin-tone-medium = Media
enum-launcher-skin-tone-medium-dark = Medio-scura
enum-launcher-skin-tone-dark = Scura

settings-launcher-run-run-command-placeholder-cmd = Il comando digitato
settings-launcher-run-shell-command-placeholder-terminal = Il terminale
settings-launcher-run-shell-command-placeholder-cmd = Il comando digitato
settings-launcher-ssh-command-placeholder-terminal = Il terminale
settings-launcher-ssh-command-placeholder-ssh-client = Il client SSH
settings-launcher-ssh-command-placeholder-host = L’host
settings-launcher-preview-cmd-placeholder-input = Il file da visualizzare in anteprima
settings-launcher-preview-cmd-placeholder-output = Dove scrivere la miniatura
settings-launcher-preview-cmd-placeholder-size = La dimensione della miniatura in pixel
