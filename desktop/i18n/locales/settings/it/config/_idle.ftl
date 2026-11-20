### lys-shell Configuration - Idle

## Idle Configuration

settings-idle-enable = Criterio di inattività integrato
    .description = Attenua, blocca, spegne gli schermi e sospende in caso di inattività; disattivalo quando l'inattività è gestita da swayidle o hypridle

settings-idle-dim-percent = Luminosità attenuata
    .description = Percentuale del proprio livello che mantiene una retroilluminazione attenuata

settings-idle-suspend-action = Modalità di sospensione
    .description = Come il sistema va a dormire quando scatta la fase di sospensione, tramite logind

settings-idle-lock-before-sleep = Blocca prima della sospensione
    .description = Blocca la sessione prima che il sistema vada in sospensione, così si riattiva sempre bloccato

settings-idle-timeouts-ac = Con alimentazione da rete
    .description = Tempo di inattività come durata, ad esempio 90s, 5m o 1h30m, oppure never

settings-idle-timeouts-battery = A batteria
    .description = Tempo di inattività come durata, ad esempio 90s, 5m o 1h30m, oppure never

## SleepAction variants
enum-sleep-action-auto = Sospensione, poi ibernazione se possibile
enum-sleep-action-suspend = Sospensione
enum-sleep-action-hibernate = Ibernazione
enum-sleep-action-hybrid-sleep = Sospensione ibrida
enum-sleep-action-suspend-then-hibernate = Sospensione, poi ibernazione
