### lys-shell Configuration - Idle

## Idle Configuration

settings-idle-enable = Eingebaute Leerlaufrichtlinie
    .description = Bei Leerlauf abdunkeln, sperren, Bildschirme ausschalten und in Bereitschaft wechseln; ausschalten, wenn swayidle oder hypridle den Leerlauf übernimmt

settings-idle-dim-percent = Abgedunkelte Helligkeit
    .description = Anteil ihrer Helligkeit in Prozent, den eine abgedunkelte Hintergrundbeleuchtung behält

settings-idle-suspend-action = Schlafmodus
    .description = Wie das System schläft, wenn die Bereitschaftsstufe greift, über logind

settings-idle-lock-before-sleep = Vor Bereitschaft sperren
    .description = Die Sitzung sperren, bevor das System in Bereitschaft geht, damit es immer gesperrt aufwacht

settings-idle-timeouts-ac = Im Netzbetrieb
    .description = Leerlaufzeit als Dauer wie 90s, 5m oder 1h30m, oder never

settings-idle-timeouts-battery = Im Akkubetrieb
    .description = Leerlaufzeit als Dauer wie 90s, 5m oder 1h30m, oder never

## SleepAction variants
enum-sleep-action-auto = Bereitschaft, dann Ruhezustand wenn möglich
enum-sleep-action-suspend = Bereitschaft
enum-sleep-action-hibernate = Ruhezustand
enum-sleep-action-hybrid-sleep = Hybrider Schlaf
enum-sleep-action-suspend-then-hibernate = Bereitschaft, dann Ruhezustand
