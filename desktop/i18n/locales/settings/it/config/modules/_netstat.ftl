### lys-shell Configuration - Netstat Module

## Netstat Module Configuration

settings-modules-netstat-poll-interval-ms = Intervallo di aggiornamento
    .description = Intervallo di aggiornamento in millisecondi

settings-modules-netstat-interface = Interfaccia
    .description = L'interfaccia di rete da monitorare: quella attiva, scelta automaticamente, o una per nome

settings-modules-netstat-format = Formato di visualizzazione
    .description = Stringa di formato con i segnaposto: {"{{ down_kib }}"}, {"{{ down_mib }}"}, {"{{ down_gib }}"}, {"{{ down_auto }}"}, {"{{ up_kib }}"}, {"{{ up_mib }}"}, {"{{ up_gib }}"}, {"{{ up_auto }}"}, {"{{ interface }}"}

settings-modules-netstat-icon-name = Nome dell'icona
    .description = Nome dell'icona simbolica

settings-modules-netstat-border-show = Mostra il bordo
    .description = Mostra un bordo attorno al pulsante

settings-modules-netstat-border-color = Colore del bordo
    .description = Token del colore del bordo

settings-modules-netstat-icon-show = Mostra l'icona
    .description = Mostra l'icona del modulo

settings-modules-netstat-tooltip = Mostra il suggerimento
    .description = Mostra lo stato del modulo in un suggerimento al passaggio del mouse

settings-modules-netstat-icon-color = Colore dell'icona
    .description = Colore di primo piano dell'icona

settings-modules-netstat-icon-bg-color = Sfondo dell'icona
    .description = Colore di sfondo del contenitore dell'icona

settings-modules-netstat-label-show = Mostra l'etichetta
    .description = Mostra l'etichetta di testo

settings-modules-netstat-label-color = Colore dell'etichetta
    .description = Colore del testo dell'etichetta

settings-modules-netstat-label-max-length = Lunghezza massima dell'etichetta
    .description = Numero massimo di caratteri prima del troncamento

settings-modules-netstat-button-bg-color = Sfondo del pulsante
    .description = Colore di sfondo del pulsante

settings-modules-netstat-left-click = Clic sinistro
    .description = Comando della shell al clic sinistro

settings-modules-netstat-right-click = Clic destro
    .description = Comando della shell al clic destro

settings-modules-netstat-middle-click = Clic centrale
    .description = Comando della shell al clic centrale

settings-modules-netstat-scroll-up = Scorrimento in alto
    .description = Comando della shell allo scorrimento in alto

settings-modules-netstat-scroll-down = Scorrimento in basso
    .description = Comando della shell allo scorrimento in basso

## Format placeholders

settings-modules-netstat-format-placeholder-down-kib = Velocità di download in KiB/s

settings-modules-netstat-format-placeholder-down-mib = Velocità di download in MiB/s

settings-modules-netstat-format-placeholder-down-gib = Velocità di download in GiB/s

settings-modules-netstat-format-placeholder-down-auto = Velocità di download con la sua unità (ad es. 1.5 MiB/s)

settings-modules-netstat-format-placeholder-up-kib = Velocità di upload in KiB/s

settings-modules-netstat-format-placeholder-up-mib = Velocità di upload in MiB/s

settings-modules-netstat-format-placeholder-up-gib = Velocità di upload in GiB/s

settings-modules-netstat-format-placeholder-up-auto = Velocità di upload con la sua unità (ad es. 256 KiB/s)

settings-modules-netstat-format-placeholder-interface = Nome dell'interfaccia (ad es. wlan0)
