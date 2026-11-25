settings-login-background-mode = Achtergrond
    .description = Hoe de achtergrond van het aanmeldscherm wordt getekend: effen kleur, een afbeelding of de bureaubladachtergrond

settings-login-background-image = Achtergrondafbeelding
    .description = Afbeeldingsbestand achter het aanmeldscherm als de achtergrond op "image" staat

settings-login-background-color = Achtergrondkleur
    .description = Vulkleur achter het aanmeldscherm als de achtergrond op "color" staat

settings-login-blur = Vervaging
    .description = Straal van de Gaussiaanse vervaging op afbeeldings-/bureaubladachtergronden (0 = geen)

settings-login-show-clock = Klok tonen
    .description = Een klok boven de aanmeldvraag tonen

settings-login-clock-format = Klokformaat
    .description = strftime-formaat voor de tijd op het aanmeldscherm (bijv. %H:%M)

settings-login-date-format = Datumformaat
    .description = strftime-formaat voor de datum op het aanmeldscherm (bijv. %A, %B %-d)

settings-login-max-attempts = Maximaal aantal pogingen
    .description = Mislukte wachtwoordpogingen op het vergrendelscherm voordat invoer wordt geblokkeerd (0 = onbeperkt). Het scherm blijft vergrendeld.

settings-login-show-failed-attempts = Mislukte pogingen tonen
    .description = Het aantal mislukte pogingen op het vergrendelscherm tonen

settings-login-lock-enabled = Ingeschakeld
    .description = lys-shell de sessie laten vergrendelen (reageert op loginctl lock-session en lys-shell lock)

settings-login-lock-lock-on-start = Vergrendelen bij start
    .description = De sessie vergrendelen zodra de shell start, voor opstellingen met automatisch aanmelden waarin het vergrendelscherm de toegangspoort is

settings-login-lock-grace-period-ms = Respijtperiode
    .description = Periode na het vergrendelen waarin het scherm zonder wachtwoord ontgrendelt (0 = altijd vereisen)

settings-login-lock-blank-timeout-ms = Time-out voor zwart scherm
    .description = Het vergrendelscherm zwart maken na deze inactiviteitstijd (0 = nooit; alleen visueel — DPMS is de taak van de inactiviteitsdaemon)

settings-login-lock-pam-service = PAM-dienst
    .description = PAM-dienst die het wachtwoord controleert (bijv. system-auth, login)

settings-login-lock-show-media = Nu afspelend tonen
    .description = Het spelende nummer met afspelen/pauzeren en volgende op het vergrendelscherm tonen (alleen lokale hoesafbeeldingen)

settings-login-lock-show-battery = Accu tonen
    .description = De lading van de accu op het vergrendelscherm tonen; afwezig zonder accu

settings-login-lock-fingerprint = Vingerafdruk
    .description = Ontgrendelen met een geregistreerde vingerafdruk (fprintd) naast het wachtwoord

settings-login-lock-show-switch-user = Gebruiker wisselen tonen
    .description = "Gebruiker wisselen" aanbieden op het vergrendelscherm als de machine meer dan één account heeft

settings-login-greeter-show-user-list = Gebruikerslijst tonen
    .description = Klikbare avatars van de aanmeldgebruikers van de machine boven het gebruikersnaamveld tonen

settings-login-greeter-show-power-buttons = Energieknoppen tonen
    .description = De knoppen uitschakelen/herstarten onderaan het aanmeldscherm tonen

settings-login-greeter-fingerprint = Aanmelden met vingerafdruk
    .description = Aanmelden met een geregistreerde vingerafdruk (fprintd) naast het wachtwoord

settings-login-greeter-match-cursor = Mijn cursor overnemen
    .description = De cursor van de laatst aangemelde sessie tonen in plaats van het thema en de grootte die hier zijn ingesteld

settings-login-greeter-cursor-theme = Cursorthema
    .description = Xcursor-thema op het aanmeldscherm (leeg = systeemstandaard)

settings-login-greeter-cursor-size = Cursorgrootte
    .description = Logische cursorgrootte; per beeldscherm geschaald zodat HiDPI-schermen een scherpe cursor krijgen

settings-login-greeter-screen-reader = Schermlezer
    .description = Schermlezer die het toegankelijkheidsmenu van het aanmeldscherm en Super+Alt+S starten; verborgen als hij niet is geïnstalleerd

enum-lock-background-color = Kleur
enum-lock-background-image = Afbeelding
enum-lock-background-wallpaper = Achtergrond
