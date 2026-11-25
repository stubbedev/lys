### lys-shell Configuration - Login Screen

## Login screen: the greeter and the lock screen

settings-login-background-mode = Background
    .description = How the login screen background is drawn: solid color, an image, or the wallpaper

settings-login-background-image = Background Image
    .description = Picture shown behind the login screen when the background is Image

settings-login-background-color = Background Color
    .description = Fill color behind the login screen when the background is Color

settings-login-blur = Blur
    .description = How strongly a picture or wallpaper background is blurred; 0 keeps it sharp

settings-login-show-clock = Show Clock
    .description = Display a clock above the credential prompt

settings-login-clock-format = Clock Format
    .description = How the time on the login screen reads

settings-login-date-format = Date Format
    .description = How the date on the login screen reads

settings-login-max-attempts = Max Attempts
    .description = Wrong passwords on the lock screen before typing is blocked, the screen still locked; 0 sets no limit

settings-login-show-failed-attempts = Show Failed Attempts
    .description = Show the failed-attempt count on the lock screen

settings-login-lock-enabled = Enabled
    .description = Let lys-shell lock the session (responds to loginctl lock-session and lys-shell lock)

settings-login-lock-lock-on-start = Lock on Start
    .description = Lock the session as soon as the shell starts, for autologin setups where the lock screen is the access gate

settings-login-lock-grace-period-ms = Grace Period
    .description = How long after locking the screen still unlocks without a password

settings-login-lock-blank-timeout-ms = Blank Timeout
    .description = Black out the lock screen after it sits idle this long; when the displays turn off is set on the Idle & Power page

settings-login-lock-pam-service = PAM Service
    .description = PAM service that checks the password, such as system-auth or login

settings-login-lock-show-media = Show Now Playing
    .description = Show the playing track with play/pause and next on the lock screen (local cover art only)

settings-login-lock-show-battery = Show Battery
    .description = Show the battery's charge on the lock screen; absent without a battery

settings-login-lock-fingerprint = Fingerprint
    .description = Unlock with an enrolled fingerprint (fprintd) alongside the password

settings-login-lock-show-switch-user = Show Switch User
    .description = Offer "Switch user" on the lock screen when the machine has more than one account

settings-login-greeter-show-user-list = Show User List
    .description = Show clickable avatars for the machine's login users above the username field

settings-login-greeter-show-power-buttons = Show Power Buttons
    .description = Show the shutdown/reboot buttons at the bottom of the login screen

settings-login-greeter-fingerprint = Fingerprint Login
    .description = Log in with an enrolled fingerprint (fprintd) as well as the password

settings-login-greeter-match-cursor = Match My Cursor
    .description = Show the cursor the last session to log in showed, instead of the theme and size set here

settings-login-greeter-cursor-theme = Cursor Theme
    .description = Cursor theme on the login screen; left unset, the system's own

settings-login-greeter-cursor-size = Cursor Size
    .description = Logical cursor size; scaled per display so HiDPI outputs get a sharp cursor

settings-login-greeter-screen-reader = Screen Reader
    .description = Screen reader the greeter's accessibility menu and Super+Alt+S start; hidden when not installed

## LockBackground variants
enum-lock-background-color = Color
enum-lock-background-image = Image
enum-lock-background-wallpaper = Wallpaper
