### lys-shell Configuration - Recorder Module

## Recorder Module Configuration

settings-modules-recorder-icon-idle = Icona inattivo
    .description = Icona quando non si registra

settings-modules-recorder-icon-recording = Icona di registrazione
    .description = Icona durante la registrazione

settings-modules-recorder-icon-paused = Icona in pausa
    .description = Icona quando la registrazione è in pausa

settings-modules-recorder-icon-replay = Icona replay
    .description = Icona quando è pronto solo il replay istantaneo

settings-modules-recorder-format = Formato
    .description = Stringa di formato con i segnaposto: {"{{ state }}"}, {"{{ elapsed }}"}

settings-modules-recorder-microphone = Microfono
    .description = Registra il microfono

settings-modules-recorder-microphone-device = Dispositivo microfono
    .description = Nome della sorgente del microfono (vuoto usa la sorgente predefinita)

settings-modules-recorder-system-audio = Audio di sistema
    .description = Registra l'audio del desktop

settings-modules-recorder-framerate = Frequenza dei fotogrammi
    .description = Frequenza di acquisizione in fotogrammi al secondo

settings-modules-recorder-webcam-enabled = Riquadro webcam
    .description = Sovrappone alla registrazione un riquadro picture-in-picture della webcam

settings-modules-recorder-webcam-device = Dispositivo webcam
    .description = Percorso del dispositivo V4L2 (vuoto seleziona automaticamente la prima fotocamera)

settings-modules-recorder-webcam-x = Posizione X della webcam
    .description = Posizione orizzontale (0% = sinistra, 100% = destra), relativa così resiste ai cambi di risoluzione

settings-modules-recorder-webcam-y = Posizione Y della webcam
    .description = Posizione verticale (0% = alto, 100% = basso), relativa così resiste ai cambi di risoluzione

settings-modules-recorder-webcam-size = Dimensione della webcam
    .description = Larghezza del riquadro della webcam come percentuale della larghezza della registrazione

settings-modules-recorder-output-directory = Cartella di destinazione
    .description = Dove vengono salvate le registrazioni (vuoto usa la cartella XDG Video)

settings-modules-recorder-output-format = Formato di uscita
    .description = Formato contenitore / preset del codec

settings-modules-recorder-show-cursor = Mostra il cursore
    .description = Disegna il cursore del mouse nella registrazione

settings-modules-recorder-start-delay-ms = Ritardo di avvio
    .description = Ritardo tra la scelta della sorgente e la registrazione, così il toast di avvio libera prima lo schermo

settings-modules-recorder-border-show = Mostra il bordo
    .description = Mostra un bordo attorno al pulsante

settings-modules-recorder-border-color = Colore del bordo
    .description = Token del colore del bordo

settings-modules-recorder-icon-show = Mostra l'icona
    .description = Mostra l'icona del modulo

settings-modules-recorder-tooltip = Mostra il suggerimento
    .description = Mostra lo stato del modulo in un suggerimento al passaggio del mouse

settings-modules-recorder-icon-color = Colore dell'icona
    .description = Colore di primo piano dell'icona

settings-modules-recorder-icon-bg-color = Sfondo dell'icona
    .description = Colore di sfondo del contenitore dell'icona

settings-modules-recorder-label-show = Mostra l'etichetta
    .description = Mostra l'etichetta

settings-modules-recorder-label-color = Colore dell'etichetta
    .description = Colore del testo dell'etichetta

settings-modules-recorder-label-max-length = Lunghezza massima dell'etichetta
    .description = Numero massimo di caratteri prima del troncamento

settings-modules-recorder-button-bg-color = Sfondo del pulsante
    .description = Colore di sfondo del pulsante

settings-modules-recorder-left-click = Clic sinistro
    .description = Comando della shell al clic sinistro

settings-modules-recorder-right-click = Clic destro
    .description = Comando della shell al clic destro

settings-modules-recorder-middle-click = Clic centrale
    .description = Comando della shell al clic centrale

settings-modules-recorder-scroll-up = Scorrimento in alto
    .description = Comando della shell allo scorrimento in alto

settings-modules-recorder-scroll-down = Scorrimento in basso
    .description = Comando della shell allo scorrimento in basso


## RecorderFormat variants
enum-recorder-format-mp4 = MP4
enum-recorder-format-mkv = MKV

## Instant replay

settings-modules-recorder-replay-enable = Replay istantaneo
    .description = Conserva in memoria gli ultimi secondi di un'uscita, pronti da salvare

settings-modules-recorder-replay-seconds = Durata del replay
    .description = Secondi di filmato scritti al salvataggio di un replay (5-600)

settings-modules-recorder-replay-output = Uscita del replay
    .description = Lo schermo acquisito: il monitor attivo quando il replay viene armato, o uno indicato dal suo connettore, come DP-1

## Format placeholders

settings-modules-recorder-format-placeholder-state = Stato del registratore (Inattivo, Replay pronto, Registrazione, In pausa)

settings-modules-recorder-format-placeholder-elapsed = Tempo di registrazione trascorso come M:SS o H:MM:SS (- se inattivo)
