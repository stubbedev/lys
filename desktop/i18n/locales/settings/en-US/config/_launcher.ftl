# Launcher (rofi replacement)

settings-launcher-location = Position
    .description = Where the launcher appears on screen

settings-launcher-width = Width
    .description = How wide the launcher is, as a multiple of its usual width or in pixels

settings-launcher-lines = Lines
    .description = Visible result lines

settings-launcher-monitor = Monitor
    .description = The display the launcher shows on

settings-launcher-modes = Modes
    .description = Enabled modes, in tab order (drun, run, window, ssh, filebrowser, combi, keys, or a script name)

settings-launcher-cycle = Cycle Selection
    .description = Wrap the selection at list edges

settings-launcher-matching = Matching Method
    .description = How typed text is matched against entries

settings-launcher-tokenize = Tokenize
    .description = Match each word of the query independently

settings-launcher-negate-char = Negation Character
    .description = A word typed after this character hides the matches that contain it

settings-launcher-normalize-match = Normalize Match
    .description = Ignore accents and Unicode variants while matching

settings-launcher-sort = Sort Results
    .description = Rank results by match quality instead of list order

settings-launcher-sorting-method = Sorting Method
    .description = Ranking algorithm used when sorting is enabled

settings-launcher-case = Case Sensitivity
    .description = How letter case affects matching

settings-launcher-terminal = Terminal
    .description = Terminal emulator for terminal apps; left empty, one installed is found

settings-launcher-show-icons = Show Icons
    .description = Show icons beside entries

settings-launcher-icon-theme = Icon Theme
    .description = Icons of the launcher's rows; Default follows the system's icon theme

settings-launcher-sidebar-mode = Sidebar Mode
    .description = Show mode tabs at the bottom of the launcher

settings-launcher-auto-select = Auto Select
    .description = Accept automatically when exactly one result remains

settings-launcher-hover-select = Hover Select
    .description = Select the row under the mouse cursor

settings-launcher-fixed-num-lines = Fixed Height
    .description = Keep the list height fixed at the configured line count

settings-launcher-display-names = Mode Display Names
    .description = Per-mode display-name overrides (e.g. drun = "apps")

settings-launcher-scripts = Script Modes
    .description = Modes of your own, each a name and the program that fills its list

settings-launcher-keybindings = Keybindings
    .description = Which keys run each launcher action; an action you leave alone keeps its usual keys

settings-launcher-mouse-bindings = Mouse Bindings
    .description = Which mouse buttons and scroll directions run each launcher action, in place of its defaults

settings-launcher-font = Font
    .description = The launcher's font and size; Default keeps the shell's font

settings-launcher-styles = Style Presets
    .description = Named looks a launch can ask for by name: each name → the CSS applied while that launch shows

settings-launcher-preview-cmd = Preview Command
    .description = Command making a row's thumbnail from the file, the image to write and its size in pixels; empty uses the system's thumbnailers

settings-launcher-history = History
    .description = Launch history and frecency ranking

settings-launcher-drun = Applications
    .description = drun (application) mode

settings-launcher-run = Commands
    .description = run (command) mode

settings-launcher-window = Windows
    .description = Window switcher mode

settings-launcher-ssh = SSH
    .description = SSH host mode

settings-launcher-filebrowser = File Browser
    .description = File browser mode

settings-launcher-combi = Combi
    .description = Combined-modes mode

settings-launcher-history-enable = Enable History
    .description = Record launches and rank frequently used entries first

settings-launcher-history-max-size = History Size
    .description = Maximum remembered entries per mode

settings-launcher-drun-categories = Categories
    .description = Only show applications within these categories; with none chosen, every application shows

settings-launcher-drun-exclude-categories = Excluded Categories
    .description = Hide applications within these categories

settings-launcher-drun-match-fields = Match Fields
    .description = Desktop-entry fields searched while typing

settings-launcher-drun-display-format = Display Format
    .description = What each app's row shows; a part in square brackets shows only when the app has its value

settings-launcher-drun-show-actions = Show Actions
    .description = Expose desktop-file actions as extra rows

settings-launcher-drun-url-launcher = URL Launcher
    .description = Command opening entries that are web links; Default uses the built-in opener

settings-launcher-run-run-command = Run Command
    .description = Command an accepted entry runs, with the typed command in it

settings-launcher-run-shell-command = Terminal Command
    .description = Command running an entry in a terminal, from the terminal and the typed command

settings-launcher-run-list-command = List Command
    .description = Extra command whose stdout lines add entries

settings-launcher-window-format = Display Format
    .description = What each window's row shows, from its workspace, app, title, name and role

settings-launcher-window-match-fields = Match Fields
    .description = Window fields searched while typing

settings-launcher-window-hide-active = Hide Active Window
    .description = Hide the currently focused window from the list

settings-launcher-window-close-on-delete = Close On Delete
    .description = Shift-Delete closes the selected window

settings-launcher-ssh-client = SSH Client
    .description = SSH client binary

settings-launcher-ssh-command = Connect Command
    .description = Command connecting to a host, from the terminal, the SSH client and the host

settings-launcher-ssh-parse-hosts = Parse /etc/hosts
    .description = Include hosts from /etc/hosts

settings-launcher-ssh-parse-known-hosts = Parse known_hosts
    .description = Include hosts from ~/.ssh/known_hosts

settings-launcher-filebrowser-directory = Start Directory
    .description = Folder the file browser opens in; left empty, your home folder

settings-launcher-filebrowser-sorting-method = Sorting
    .description = File ordering

settings-launcher-filebrowser-directories-first = Directories First
    .description = List directories before files

settings-launcher-filebrowser-show-hidden = Show Hidden
    .description = Show hidden files

settings-launcher-filebrowser-command = Open Command
    .description = Command opening the picked file; Default uses the built-in opener

settings-launcher-combi-modes = Combined Modes
    .description = Modes merged into the combined list

settings-launcher-combi-display-format = Display Format
    .description = What each row shows, from its mode and its text

settings-launcher-combi-files = Find Files
    .description = Files found by name in the combined list, recently used ones first

## LauncherLocation variants
enum-launcher-location-center = Center
enum-launcher-location-north-west = Top Left
enum-launcher-location-north = Top
enum-launcher-location-north-east = Top Right
enum-launcher-location-east = Right
enum-launcher-location-south-east = Bottom Right
enum-launcher-location-south = Bottom
enum-launcher-location-south-west = Bottom Left
enum-launcher-location-west = Left

## LauncherMatching variants
enum-launcher-matching-normal = Normal
enum-launcher-matching-regex = Regex
enum-launcher-matching-glob = Glob
enum-launcher-matching-fuzzy = Fuzzy
enum-launcher-matching-prefix = Prefix

## LauncherSorting variants
enum-launcher-sorting-levenshtein = Levenshtein
enum-launcher-sorting-fzf = FZF

## LauncherCase variants
enum-launcher-case-insensitive = Insensitive
enum-launcher-case-smart = Smart
enum-launcher-case-sensitive = Sensitive

## LauncherDrunField variants
enum-launcher-drun-field-name = Name
enum-launcher-drun-field-generic = Generic Name
enum-launcher-drun-field-exec = Command
enum-launcher-drun-field-categories = Categories
enum-launcher-drun-field-comment = Comment
enum-launcher-drun-field-keywords = Keywords

## LauncherWindowField variants
enum-launcher-window-field-title = Title
enum-launcher-window-field-class = Class
enum-launcher-window-field-name = Name
enum-launcher-window-field-role = Role
enum-launcher-window-field-desktop = Workspace

## LauncherFileSort variants
enum-launcher-file-sort-name = Name
enum-launcher-file-sort-mtime = Modified
enum-launcher-file-sort-atime = Accessed
enum-launcher-file-sort-ctime = Created

## Animations
settings-animations-launcher = Launcher
    .description = Enter/exit animation override for the application launcher

# Search panel

settings-launcher-search = Search Panel
    .description = The panel lys-shell launcher opens with no arguments

settings-launcher-search-providers = Sources
    .description = What the list searches

settings-launcher-search-web-engine = Web Search Name
    .description = The engine's name on the fallback row

settings-launcher-search-web-url = Web Search URL
    .description = The URL, with {"{"}query{"}"} where the search goes

settings-launcher-search-file-roots = File Index Folders
    .description = Folders the file index walks; empty indexes the home folder

settings-launcher-search-file-exclude = File Index Exclusions
    .description = Folder names the file index skips

settings-launcher-search-commands-dir = Script Commands Folder
    .description = Empty uses launcher/commands in the config folder

settings-launcher-search-currency-rates = Currency Rates
    .description = Fetch the European Central Bank's daily reference rates, at most once a day, for currency answers

settings-launcher-search-previews = Previews
    .description = Show the side pane for files, snippets, windows and clipboard entries, in the search panel and the window and clipboard modes

settings-launcher-search-quicklinks = Quick Links
    .description = Links to open by name or keyword

settings-launcher-search-snippets = Snippets
    .description = Text to copy by name or keyword

enum-launcher-search-provider-apps = Applications
enum-launcher-search-provider-windows = Open Windows
enum-launcher-search-provider-files = Files
enum-launcher-search-provider-calculator = Calculator
enum-launcher-search-provider-system = System Commands
enum-launcher-search-provider-settings = Settings Pages
enum-launcher-search-provider-clipboard = Clipboard History
enum-launcher-search-provider-emoji = Emoji
enum-launcher-search-provider-characters = Unicode Characters
enum-launcher-search-provider-vault = Bitwarden Vault
enum-launcher-search-provider-links = Quick Links and Snippets
enum-launcher-search-provider-browser-bookmarks = Browser Bookmarks
enum-launcher-search-provider-browser-history = Browser History
enum-launcher-search-provider-quick-toggles = Quick Toggles
enum-launcher-search-provider-media = Media Controls
enum-launcher-search-provider-capture = Screenshots and Recording
enum-launcher-search-provider-processes = Kill Process
enum-launcher-search-provider-scripts = Script Commands
enum-launcher-search-provider-shell = Shell Commands
enum-launcher-search-provider-web = Web Search

## Format placeholders

settings-launcher-drun-display-format-placeholder-name = Application name

settings-launcher-drun-display-format-placeholder-generic = Generic name (e.g. Web Browser)

settings-launcher-drun-display-format-placeholder-exec = Command the entry runs

settings-launcher-drun-display-format-placeholder-categories = Desktop entry categories

settings-launcher-drun-display-format-placeholder-comment = Desktop entry comment

settings-launcher-window-format-placeholder-w = Workspace

settings-launcher-window-format-placeholder-c = Window class

settings-launcher-window-format-placeholder-t = Window title

settings-launcher-window-format-placeholder-n = Window name (its class)

settings-launcher-window-format-placeholder-r = Window role (always empty)

settings-launcher-combi-display-format-placeholder-mode = Mode the row comes from

settings-launcher-combi-display-format-placeholder-text = The row's own text

# Emoji

settings-launcher-emoji = Emoji
    .description = Emoji in the search panel, the emoji grid and the emoji mode

settings-launcher-emoji-skin-tone = Skin Tone
    .description = The skin tone of person and hand emoji; picking one from an emoji's actions makes it the default

## LauncherSkinTone variants
enum-launcher-skin-tone-none = Default (Yellow)
enum-launcher-skin-tone-light = Light
enum-launcher-skin-tone-medium-light = Medium-Light
enum-launcher-skin-tone-medium = Medium
enum-launcher-skin-tone-medium-dark = Medium-Dark
enum-launcher-skin-tone-dark = Dark

settings-launcher-run-run-command-placeholder-cmd = The command typed
settings-launcher-run-shell-command-placeholder-terminal = The terminal
settings-launcher-run-shell-command-placeholder-cmd = The command typed
settings-launcher-ssh-command-placeholder-terminal = The terminal
settings-launcher-ssh-command-placeholder-ssh-client = The SSH client
settings-launcher-ssh-command-placeholder-host = The host
settings-launcher-preview-cmd-placeholder-input = The file to preview
settings-launcher-preview-cmd-placeholder-output = Where to write the thumbnail
settings-launcher-preview-cmd-placeholder-size = The thumbnail size in pixels

## Quick links and snippets

settings-launcher-quicklink-name = Name
settings-launcher-quicklink-url = Address
settings-launcher-quicklink-keyword = Keyword
settings-launcher-quicklink-icon = Icon
settings-launcher-quicklink-url-placeholder-query = The words typed after the keyword
settings-launcher-snippet-name = Name
settings-launcher-snippet-text = Text
settings-launcher-snippet-keyword = Keyword
