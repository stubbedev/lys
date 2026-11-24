### Battery Dropdown

dropdown-battery-title = Batteria

## Hero States
dropdown-battery-on-battery = A batteria
dropdown-battery-charging = In carica
dropdown-battery-plugged-in = Collegato
dropdown-battery-not-charging = Collegato, non in carica
dropdown-battery-critical = Critico

## Time Display
dropdown-battery-duration-hm = { $hours } h { $minutes } min
dropdown-battery-duration-m = { $minutes } min
dropdown-battery-time-remaining = { $duration } rimanenti
dropdown-battery-time-until-full = { $duration } alla carica completa

## Details
dropdown-battery-draw = Consumo
dropdown-battery-input = Ingresso
dropdown-battery-input-watts = { $watts } in ingresso
dropdown-battery-capacity = Capacità
dropdown-battery-charged = Carica
dropdown-battery-health = Stato di salute

## Charge Limit
dropdown-battery-charge-limit = Limite di carica
dropdown-battery-limit-to = Limita al { $threshold }%
dropdown-battery-charge-limit-not-supported = Limite di carica non supportato su questo dispositivo
dropdown-battery-health-cycles = Salute, { $cycles ->
    [one] { $cycles } ciclo
   *[other] { $cycles } cicli
}
dropdown-battery-hold = Mantenuta al { $limit }% per proteggere la batteria
dropdown-battery-hold-until = Mantenuta al { $limit }% per proteggere la batteria; piena alle { $time }
dropdown-battery-hold-off = In carica al 100%: nessun limite di carica
dropdown-battery-full-requested = In carica al 100% finché non scolleghi
dropdown-battery-full-unplug = In carica al 100% per quando di solito scolleghi, piena alle { $time }
dropdown-battery-full-event = In carica al 100% per il tuo evento alle { $time }
dropdown-battery-charge-full = Carica completa ora
dropdown-battery-charge-full-failed = Impossibile avviare la carica completa

## Power Profile
dropdown-battery-power-profile = Profilo energetico
dropdown-battery-profile-saver = Risparmio
dropdown-battery-profile-balanced = Bilanciato
dropdown-battery-profile-performance = Prestazioni
dropdown-battery-power-profile-not-available = Il demone dei profili energetici deve essere attivo

## No Battery
dropdown-battery-no-battery-title = Nessuna batteria rilevata
dropdown-battery-no-battery-description = Alimentazione da rete elettrica

## Peripherals
dropdown-battery-devices = Dispositivi

## Significant Energy
dropdown-battery-energy = Usano molta energia
dropdown-battery-energy-none = Nessuna app usa molta energia
dropdown-battery-energy-measuring = Misurazione in corso…
dropdown-battery-energy-cpu = { $percent }% di una CPU
dropdown-battery-energy-watts = { $watts }, { $percent }% di una CPU
dropdown-battery-energy-show = Mostra
dropdown-battery-energy-quit = Esci
dropdown-battery-energy-quit-failed = Impossibile chiudere { $app }
dropdown-battery-energy-show-failed = Impossibile mostrare { $app }
dropdown-battery-paused = In pausa a batteria
dropdown-battery-paused-now = In pausa finché non colleghi la corrente
dropdown-battery-paused-failed = Impossibile mettere in pausa
dropdown-battery-paused-resume-failed = Impossibile riprendere

## Alerts
battery-alert-low = Batteria scarica
battery-alert-critical = Batteria criticamente scarica
battery-alert-remaining = { $percent }% rimanente
battery-alert-charged = Batteria carica
battery-alert-charged-body = Carica al { $percent }%, puoi scollegare il caricatore
battery-alert-device-low = Batteria di { $device } scarica
battery-alert-device-critical = Batteria di { $device } criticamente scarica
battery-offer-low-power = Attivare il risparmio energetico?
battery-offer-low-power-body = { $percent }% rimanente. Il risparmio energetico riduce la frequenza di aggiornamento, attenua leggermente lo schermo e interroga meno spesso finché non colleghi il caricatore.
battery-offer-low-power-action = Attiva
battery-sleep-drained = L’ultima sospensione ha consumato più batteria del dovuto
battery-sleep-drained-body = Ha assorbito { $watts } W in { $hours } h di sospensione, solo il { $percent }% nello stato più profondo.
battery-sleep-blocked = Tenuta fuori dal sonno profondo da { $devices }
battery-sleep-woken = Risvegliata da { $devices }
battery-paused = In pausa a batteria
battery-paused-body = In pausa finché non colleghi la corrente: { $units }
battery-paused-resume = Riprendi per questa sessione
battery-paused-keep = Lascia in esecuzione a batteria

## Peripheral Kinds
battery-device-generic = Dispositivo
battery-device-mouse = Mouse
battery-device-touchpad = Touchpad
battery-device-keyboard = Tastiera
battery-device-controller = Controller
battery-device-pen = Penna
