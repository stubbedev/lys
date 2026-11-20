### Configuration lys-shell - Inactivité

## Configuration de l'inactivité

settings-idle-enable = Gestion intégrée de l'inactivité
    .description = Assombrir, verrouiller, éteindre les écrans et mettre en veille en cas d'inactivité ; désactivez-la si swayidle ou hypridle s'en charge

settings-idle-dim-percent = Luminosité assombrie
    .description = Part de son niveau, en pourcentage, que garde un rétroéclairage assombri

settings-idle-suspend-action = Mode de veille
    .description = Comment le système se met en veille à l’étape de mise en veille, via logind

settings-idle-lock-before-sleep = Verrouiller avant la veille
    .description = Verrouiller la session avant la mise en veille, pour qu'elle se réveille toujours verrouillée

settings-idle-timeouts-ac = Sur secteur
    .description = Durée d'inactivité, par exemple 90s, 5m ou 1h30m, ou never

settings-idle-timeouts-battery = Sur batterie
    .description = Durée d'inactivité, par exemple 90s, 5m ou 1h30m, ou never

settings-idle-attention = Regarder avant d'assombrir
    .description = Avant que l'écran ne s'assombrisse, allumer la caméra moins d'une seconde, la caméra infrarouge d'abord, pour voir si un visage est tourné vers l'écran : un visage reporte l'assombrissement. Son voyant s'allume pendant qu'elle regarde. L'image sert seulement à chercher un visage puis est jetée, jamais enregistrée, journalisée ni envoyée, et rien n'est regardé tant que la session est verrouillée ou qu'une autre application utilise la caméra

## SleepAction variants
enum-sleep-action-auto = Veille, puis hibernation si possible
enum-sleep-action-suspend = Veille
enum-sleep-action-hibernate = Hibernation
enum-sleep-action-hybrid-sleep = Veille hybride
enum-sleep-action-suspend-then-hibernate = Veille, puis hibernation

## IdleAttention variants
enum-idle-attention-off = Désactivé
enum-idle-attention-postpone-dim = Reporter seulement l'assombrissement
enum-idle-attention-sooner-when-absent = Éteindre aussi l'écran plus tôt en l'absence de quelqu'un
