### Battery Dropdown

dropdown-battery-title = Akku

## Hero States
dropdown-battery-on-battery = Akkubetrieb
dropdown-battery-charging = Wird geladen
dropdown-battery-plugged-in = Angeschlossen
dropdown-battery-not-charging = Angeschlossen, wird nicht geladen
dropdown-battery-critical = Kritisch

## Time Display
dropdown-battery-duration-hm = { $hours } h { $minutes } min
dropdown-battery-duration-m = { $minutes } min
dropdown-battery-time-remaining = { $duration } verbleibend
dropdown-battery-time-until-full = { $duration } bis voll

## Details
dropdown-battery-draw = Verbrauch
dropdown-battery-input = Eingang
dropdown-battery-input-watts = { $watts } Eingang
dropdown-battery-capacity = Kapazität
dropdown-battery-charged = Geladen
dropdown-battery-health = Zustand

## Charge Limit
dropdown-battery-charge-limit = Ladegrenze
dropdown-battery-limit-to = Auf { $threshold } % begrenzen
dropdown-battery-resumes-at = Laden wird bei { $threshold } % fortgesetzt
dropdown-battery-charge-limit-not-supported = Ladegrenze wird auf diesem Gerät nicht unterstützt
dropdown-battery-charge-limit-failed = Ladegrenze konnte nicht geändert werden
dropdown-battery-health-cycles = Zustand, { $cycles ->
    [one] { $cycles } Ladezyklus
   *[other] { $cycles } Ladezyklen
}
dropdown-battery-hold = Hält { $limit } %, um den Akku zu schonen
dropdown-battery-hold-until = Hält { $limit } %, um den Akku zu schonen; voll um { $time }
dropdown-battery-hold-off = Lädt auf 100 %: keine Ladegrenze gesetzt
dropdown-battery-full-requested = Lädt auf 100 %, bis du den Stecker ziehst
dropdown-battery-full-unplug = Lädt auf 100 % für dein übliches Abstecken, voll um { $time }
dropdown-battery-full-event = Lädt auf 100 % für deinen Termin um { $time }
dropdown-battery-full-travel = Lädt auf 100 % fern vom Schreibtisch
dropdown-battery-charge-full = Jetzt voll laden
dropdown-battery-charge-full-failed = Volles Laden konnte nicht gestartet werden

## Power Profile
dropdown-battery-power-profile = Energieprofil
dropdown-battery-profile-saver = Energiesparen
dropdown-battery-profile-balanced = Ausgewogen
dropdown-battery-profile-performance = Leistung
dropdown-battery-power-profile-not-available = Der Energieprofil-Dienst muss aktiv sein

## No Battery
dropdown-battery-no-battery-title = Kein Akku erkannt
dropdown-battery-no-battery-description = Netzbetrieb

## Peripherals
dropdown-battery-devices = Geräte

## Significant Energy
dropdown-battery-energy = Hoher Energieverbrauch
dropdown-battery-energy-none = Keine App verbraucht viel Energie
dropdown-battery-energy-measuring = Wird gemessen…
dropdown-battery-energy-cpu = { $percent } % einer CPU
dropdown-battery-energy-watts = { $watts }, { $percent } % einer CPU
dropdown-battery-energy-show = Anzeigen
dropdown-battery-energy-quit = Beenden
dropdown-battery-energy-quit-failed = { $app } konnte nicht beendet werden: { $error }
dropdown-battery-paused = Im Akkubetrieb pausiert
dropdown-battery-paused-now = Pausiert, bis Sie den Netzstecker einstecken
dropdown-battery-paused-failed = Konnte nicht pausieren: { $error }
dropdown-battery-paused-resume-failed = Konnte nicht fortsetzen: { $error }

## Alerts
battery-alert-low = Akku schwach
battery-alert-critical = Akku kritisch schwach
battery-alert-remaining = { $percent } % verbleibend
battery-alert-charged = Akku geladen
battery-alert-charged-body = Auf { $percent } % geladen, Sie können das Ladegerät trennen
battery-alert-device-low = { $device }: Akku schwach
battery-alert-device-critical = { $device }: Akku kritisch schwach
battery-offer-low-power = Energiesparmodus einschalten?
battery-offer-low-power-body = { $percent } % verbleibend. Der Energiesparmodus senkt die Bildwiederholrate, dimmt den Bildschirm leicht und fragt seltener ab, bis Sie das Ladegerät anschließen.
battery-offer-low-power-action = Einschalten
battery-sleep-drained = Der letzte Schlaf hat mehr Akku verbraucht, als er sollte
battery-sleep-drained-body = Er zog { $watts } W über { $hours } h Schlaf, davon nur { $percent } % im tiefsten Schlafzustand.
battery-sleep-blocked = Vom Tiefschlaf abgehalten durch { $devices }
battery-sleep-woken = Geweckt durch { $devices }
battery-paused = Im Akkubetrieb pausiert
battery-paused-body = Pausiert, bis Sie den Netzstecker einstecken: { $units }
battery-paused-resume = Für diese Sitzung fortsetzen
battery-paused-keep = Im Akkubetrieb weiterlaufen lassen

## Peripheral Kinds
battery-device-generic = Gerät
battery-device-mouse = Maus
battery-device-touchpad = Touchpad
battery-device-keyboard = Tastatur
battery-device-controller = Controller
battery-device-headset = Headset
battery-device-headphones = Kopfhörer
battery-device-speakers = Lautsprecher
battery-device-phone = Telefon
battery-device-tablet = Tablet
battery-device-pen = Stift
battery-device-watch = Uhr
battery-device-ups = USV
battery-device-printer = Drucker
battery-device-camera = Kamera
battery-device-computer = Computer
battery-device-monitor = Monitor
