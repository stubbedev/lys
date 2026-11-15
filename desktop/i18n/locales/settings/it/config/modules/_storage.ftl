### lys-shell Configuration - Storage Module

## Storage Module Configuration

settings-modules-storage-poll-interval-ms = Intervallo di aggiornamento
    .description = Intervallo di aggiornamento in millisecondi

settings-modules-storage-mount-point = Punti di montaggio
    .description = Punti di montaggio da monitorare (ad es. "/" o ["/", "/mnt/drive1"])

settings-modules-storage-format = Formato di visualizzazione
    .description = Stringa di formato con i segnaposto: {"{{ percent }}"}, {"{{ used_tib }}"}, {"{{ used_gib }}"}, {"{{ used_mib }}"}, {"{{ used_auto }}"}, {"{{ total_tib }}"}, {"{{ total_gib }}"}, {"{{ total_mib }}"}, {"{{ total_auto }}"}, {"{{ free_tib }}"}, {"{{ free_gib }}"}, {"{{ free_mib }}"}, {"{{ free_auto }}"}, {"{{ filesystem }}"}

settings-modules-storage-icon-name = Nome dell'icona
    .description = Nome dell'icona simbolica

settings-modules-storage-border-show = Mostra il bordo
    .description = Mostra un bordo attorno al pulsante

settings-modules-storage-border-color = Colore del bordo
    .description = Token del colore del bordo

settings-modules-storage-icon-show = Mostra l'icona
    .description = Mostra l'icona del modulo

settings-modules-storage-tooltip = Mostra il suggerimento
    .description = Mostra lo stato del modulo in un suggerimento al passaggio del mouse

settings-modules-storage-icon-color = Colore dell'icona
    .description = Colore di primo piano dell'icona

settings-modules-storage-icon-bg-color = Sfondo dell'icona
    .description = Colore di sfondo del contenitore dell'icona

settings-modules-storage-label-show = Mostra l'etichetta
    .description = Mostra l'etichetta di testo

settings-modules-storage-label-color = Colore dell'etichetta
    .description = Colore del testo dell'etichetta

settings-modules-storage-label-max-length = Lunghezza massima dell'etichetta
    .description = Numero massimo di caratteri prima del troncamento

settings-modules-storage-button-bg-color = Sfondo del pulsante
    .description = Colore di sfondo del pulsante

settings-modules-storage-left-click = Clic sinistro
    .description = Comando della shell al clic sinistro

settings-modules-storage-right-click = Clic destro
    .description = Comando della shell al clic destro

settings-modules-storage-middle-click = Clic centrale
    .description = Comando della shell al clic centrale

settings-modules-storage-scroll-up = Scorrimento in alto
    .description = Comando della shell allo scorrimento in alto

settings-modules-storage-scroll-down = Scorrimento in basso
    .description = Comando della shell allo scorrimento in basso

settings-modules-storage-thresholds = Soglie
    .description = Soglie di colore che cambiano in base ai livelli del valore

## Format placeholders

settings-modules-storage-format-placeholder-percent = Utilizzo del disco (0-100)

settings-modules-storage-format-placeholder-used-tib = Spazio usato in TiB

settings-modules-storage-format-placeholder-used-gib = Spazio usato in GiB

settings-modules-storage-format-placeholder-used-mib = Spazio usato in MiB

settings-modules-storage-format-placeholder-used-auto = Spazio usato con la sua unità (ad es. 128.5 GiB)

settings-modules-storage-format-placeholder-total-tib = Spazio totale in TiB

settings-modules-storage-format-placeholder-total-gib = Spazio totale in GiB

settings-modules-storage-format-placeholder-total-mib = Spazio totale in MiB

settings-modules-storage-format-placeholder-total-auto = Spazio totale con la sua unità

settings-modules-storage-format-placeholder-free-tib = Spazio libero in TiB

settings-modules-storage-format-placeholder-free-gib = Spazio libero in GiB

settings-modules-storage-format-placeholder-free-mib = Spazio libero in MiB

settings-modules-storage-format-placeholder-free-auto = Spazio libero con la sua unità

settings-modules-storage-format-placeholder-filesystem = Tipo di filesystem (ad es. ext4, btrfs)
