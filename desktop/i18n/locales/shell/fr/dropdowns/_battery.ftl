### Menu déroulant de la batterie

dropdown-battery-title = Batterie

## États principaux
dropdown-battery-on-battery = Sur batterie
dropdown-battery-charging = En charge
dropdown-battery-plugged-in = Branché
dropdown-battery-not-charging = Branché, pas en charge
dropdown-battery-critical = Critique

## Affichage de la durée
dropdown-battery-duration-hm = { $hours } h { $minutes } min
dropdown-battery-duration-m = { $minutes } min
dropdown-battery-time-remaining = { $duration } restant
dropdown-battery-time-until-full = { $duration } avant la charge complète

## Détails
dropdown-battery-draw = Consommation
dropdown-battery-input = Entrée
dropdown-battery-input-watts = { $watts } en entrée
dropdown-battery-capacity = Capacité
dropdown-battery-charged = Chargé
dropdown-battery-health = Santé

## Limite de charge
dropdown-battery-charge-limit = Limite de charge
dropdown-battery-limit-to = Limiter à { $threshold } %
dropdown-battery-resumes-at = Reprend la charge à { $threshold } %
dropdown-battery-charge-limit-not-supported = Limite de charge non prise en charge sur cet appareil
dropdown-battery-charge-limit-failed = Impossible de modifier la limite de charge
dropdown-battery-health-cycles = Santé, { $cycles ->
    [one] { $cycles } cycle
   *[other] { $cycles } cycles
}
dropdown-battery-hold = Maintenue à { $limit } % pour protéger la batterie
dropdown-battery-hold-until = Maintenue à { $limit } % pour protéger la batterie ; pleine à { $time }
dropdown-battery-hold-off = Charge à 100 % : aucune limite de charge
dropdown-battery-full-requested = Charge à 100 % jusqu'à ce que vous débranchiez
dropdown-battery-full-unplug = Charge à 100 % pour l'heure où vous débranchez d'habitude, pleine à { $time }
dropdown-battery-full-event = Charge à 100 % pour votre événement à { $time }
dropdown-battery-charge-full = Charger complètement maintenant
dropdown-battery-charge-full-failed = Impossible de lancer la charge complète

## Profil d'alimentation
dropdown-battery-power-profile = Profil d'alimentation
dropdown-battery-profile-saver = Économie
dropdown-battery-profile-balanced = Équilibré
dropdown-battery-profile-performance = Performance
dropdown-battery-power-profile-not-available = Le démon de profils d'alimentation doit être actif

## Aucune batterie
dropdown-battery-no-battery-title = Aucune batterie détectée
dropdown-battery-no-battery-description = Alimentation sur secteur

## Peripherals
dropdown-battery-devices = Appareils

## Significant Energy
dropdown-battery-energy = Utilisent beaucoup d’énergie
dropdown-battery-energy-none = Aucune app n’utilise beaucoup d’énergie
dropdown-battery-energy-measuring = Mesure en cours…
dropdown-battery-energy-cpu = { $percent } % d’un processeur
dropdown-battery-energy-watts = { $watts }, { $percent } % d’un processeur
dropdown-battery-energy-show = Afficher
dropdown-battery-energy-quit = Quitter
dropdown-battery-energy-quit-failed = Impossible de quitter { $app }
dropdown-battery-energy-show-failed = Impossible d'afficher { $app }
dropdown-battery-paused = En pause sur batterie
dropdown-battery-paused-now = En pause jusqu’au branchement
dropdown-battery-paused-failed = Mise en pause impossible
dropdown-battery-paused-resume-failed = Reprise impossible

## Alerts
battery-alert-low = Batterie faible
battery-alert-critical = Batterie très faible
battery-alert-remaining = { $percent } % restants
battery-alert-charged = Batterie chargée
battery-alert-charged-body = Chargée à { $percent } %, vous pouvez débrancher le chargeur
battery-alert-device-low = Batterie faible : { $device }
battery-alert-device-critical = Batterie très faible : { $device }
battery-offer-low-power = Activer le mode économie d’énergie ?
battery-offer-low-power-body = { $percent } % restants. Le mode économie d’énergie réduit la fréquence de rafraîchissement, assombrit légèrement l’écran et interroge moins souvent jusqu’au branchement du chargeur.
battery-offer-low-power-action = Activer
battery-sleep-drained = La dernière veille a consommé plus de batterie que prévu
battery-sleep-drained-body = Elle a tiré { $watts } W pendant { $hours } h de veille, dont seulement { $percent } % dans l’état de veille le plus profond.
battery-sleep-blocked = Empêchée de dormir profondément par { $devices }
battery-sleep-woken = Réveillée par { $devices }
battery-paused = En pause sur batterie
battery-paused-body = En pause jusqu’au branchement : { $units }
battery-paused-resume = Reprendre pour cette session
battery-paused-keep = Laisser tourner sur batterie

## Peripheral Kinds
battery-device-generic = Appareil
battery-device-mouse = Souris
battery-device-touchpad = Pavé tactile
battery-device-keyboard = Clavier
battery-device-controller = Manette
battery-device-headset = Casque micro
battery-device-headphones = Casque
battery-device-speakers = Haut-parleurs
battery-device-phone = Téléphone
battery-device-tablet = Tablette
battery-device-pen = Stylet
battery-device-watch = Montre
battery-device-ups = Onduleur
battery-device-printer = Imprimante
battery-device-camera = Appareil photo
battery-device-computer = Ordinateur
battery-device-monitor = Écran
