settings-launcher-location = Position
    .description = Emplacement du lanceur à l'écran

settings-launcher-width = Largeur
    .description = La largeur du lanceur, en multiple de sa largeur habituelle ou en pixels

settings-launcher-lines = Lignes
    .description = Nombre de lignes de résultats visibles

settings-launcher-monitor = Moniteur
    .description = L'écran sur lequel le lanceur s'affiche

settings-launcher-modes = Modes
    .description = Modes activés, dans l'ordre des onglets (drun, run, window, ssh, filebrowser, combi, keys ou un nom de script)

settings-launcher-cycle = Sélection cyclique
    .description = Reboucler la sélection aux extrémités de la liste

settings-launcher-matching = Méthode de correspondance
    .description = Manière de comparer le texte saisi aux entrées

settings-launcher-tokenize = Découper en mots
    .description = Faire correspondre chaque mot de la requête indépendamment

settings-launcher-negate-char = Caractère de négation
    .description = Un mot tapé après ce caractère masque les correspondances qui le contiennent

settings-launcher-normalize-match = Normaliser la correspondance
    .description = Ignorer les accents et les variantes Unicode lors de la correspondance

settings-launcher-sort = Trier les résultats
    .description = Classer les résultats par qualité de correspondance plutôt que dans l'ordre de la liste

settings-launcher-sorting-method = Méthode de tri
    .description = Algorithme de classement utilisé lorsque le tri est activé

settings-launcher-case = Sensibilité à la casse
    .description = Effet de la casse sur la correspondance

settings-launcher-terminal = Terminal
    .description = Émulateur de terminal pour les applications en terminal ; laissé vide, un émulateur installé est recherché

settings-launcher-show-icons = Afficher les icônes
    .description = Afficher une icône à côté des entrées

settings-launcher-icon-theme = Thème d'icônes
    .description = Icônes des lignes du lanceur ; Par défaut suit le thème d'icônes du système

settings-launcher-sidebar-mode = Barre des modes
    .description = Afficher les onglets de modes en bas du lanceur

settings-launcher-auto-select = Sélection automatique
    .description = Valider automatiquement lorsqu'il ne reste qu'un seul résultat

settings-launcher-hover-select = Sélection au survol
    .description = Sélectionner la ligne sous le curseur de la souris

settings-launcher-fixed-num-lines = Hauteur fixe
    .description = Garder la hauteur de la liste fixée au nombre de lignes configuré

settings-launcher-display-names = Noms d'affichage des modes
    .description = Noms d'affichage remplacés par mode (ex. : drun = "apps")

settings-launcher-scripts = Modes de script
    .description = Vos propres modes, chacun un nom et le programme qui remplit sa liste

settings-launcher-keybindings = Raccourcis clavier
    .description = Les touches qui lancent chaque action du lanceur ; une action non modifiée garde ses touches habituelles

settings-launcher-mouse-bindings = Raccourcis de souris
    .description = Quels boutons de souris et sens de défilement déclenchent chaque action du lanceur, à la place de ceux par défaut

settings-launcher-font = Police
    .description = Police et taille du lanceur ; Par défaut garde la police du shell

settings-launcher-styles = Préréglages de style
    .description = Apparences nommées qu'un lancement peut demander par leur nom : chaque nom → le CSS appliqué tant que ce lancement s'affiche

settings-launcher-preview-cmd = Commande d'aperçu
    .description = Commande qui crée la miniature d'une ligne à partir du fichier, de l'image à écrire et de sa taille en pixels ; vide utilise les générateurs de miniatures du système

settings-launcher-history = Historique
    .description = Historique des lancements et classement par fréquence d'utilisation

settings-launcher-drun = Applications
    .description = Mode drun (applications)

settings-launcher-run = Commandes
    .description = Mode run (commandes)

settings-launcher-window = Fenêtres
    .description = Mode de changement de fenêtre

settings-launcher-ssh = SSH
    .description = Mode des hôtes SSH

settings-launcher-filebrowser = Explorateur de fichiers
    .description = Mode d'exploration de fichiers

settings-launcher-combi = Combi
    .description = Mode combinant plusieurs modes

settings-launcher-history-enable = Activer l'historique
    .description = Enregistrer les lancements et placer en tête les entrées les plus utilisées

settings-launcher-history-max-size = Taille de l'historique
    .description = Nombre maximal d'entrées mémorisées par mode

settings-launcher-drun-categories = Catégories
    .description = N'afficher que les applications de ces catégories ; si aucune n'est choisie, toutes s'affichent

settings-launcher-drun-exclude-categories = Catégories exclues
    .description = Masquer les applications de ces catégories

settings-launcher-drun-match-fields = Champs de correspondance
    .description = Champs des entrées de bureau parcourus pendant la saisie

settings-launcher-drun-display-format = Format d'affichage
    .description = Ce qu'affiche la ligne de chaque application ; une partie entre crochets n'apparaît que si l'application en a la valeur

settings-launcher-drun-show-actions = Afficher les actions
    .description = Montrer les actions des fichiers desktop comme lignes supplémentaires

settings-launcher-drun-url-launcher = Lanceur d'URL
    .description = Commande qui ouvre les entrées qui sont des liens web ; Par défaut utilise l'ouvreur intégré

settings-launcher-run-run-command = Commande d'exécution
    .description = Commande que lance une entrée validée, avec la commande saisie dedans

settings-launcher-run-shell-command = Commande de terminal
    .description = Commande qui lance une entrée dans un terminal, à partir du terminal et de la commande saisie

settings-launcher-run-list-command = Commande de liste
    .description = Commande supplémentaire dont chaque ligne de sortie standard ajoute une entrée

settings-launcher-window-format = Format d'affichage
    .description = Ce qu'affiche la ligne de chaque fenêtre, à partir de son espace de travail, son application, son titre, son nom et son rôle

settings-launcher-window-match-fields = Champs de correspondance
    .description = Champs des fenêtres parcourus pendant la saisie

settings-launcher-window-hide-active = Masquer la fenêtre active
    .description = Retirer de la liste la fenêtre qui a le focus

settings-launcher-window-close-on-delete = Fermer avec Suppr
    .description = Maj+Suppr ferme la fenêtre sélectionnée

settings-launcher-ssh-client = Client SSH
    .description = Exécutable du client SSH

settings-launcher-ssh-command = Commande de connexion
    .description = Commande qui se connecte à un hôte, à partir du terminal, du client SSH et de l'hôte

settings-launcher-ssh-parse-hosts = Lire /etc/hosts
    .description = Inclure les hôtes de /etc/hosts

settings-launcher-ssh-parse-known-hosts = Lire known_hosts
    .description = Inclure les hôtes de ~/.ssh/known_hosts

settings-launcher-filebrowser-directory = Dossier de départ
    .description = Dossier où s'ouvre l'explorateur de fichiers ; laissé vide, votre dossier personnel

settings-launcher-filebrowser-sorting-method = Tri
    .description = Ordre des fichiers

settings-launcher-filebrowser-directories-first = Dossiers en premier
    .description = Lister les dossiers avant les fichiers

settings-launcher-filebrowser-show-hidden = Afficher les fichiers cachés
    .description = Afficher les fichiers cachés dans la liste

settings-launcher-filebrowser-command = Commande d'ouverture
    .description = Commande qui ouvre le fichier choisi ; Par défaut utilise l'ouvreur intégré

settings-launcher-combi-modes = Modes combinés
    .description = Modes fusionnés dans la liste combinée

settings-launcher-combi-display-format = Format d'affichage
    .description = Ce qu'affiche chaque ligne, à partir de son mode et de son texte

settings-launcher-combi-files = Trouver des fichiers
    .description = Fichiers trouvés par leur nom dans la liste combinée, les plus récents en premier

enum-launcher-location-center = Centre
enum-launcher-location-north-west = Haut gauche
enum-launcher-location-north = Haut
enum-launcher-location-north-east = Haut droite
enum-launcher-location-east = Droite
enum-launcher-location-south-east = Bas droite
enum-launcher-location-south = Bas
enum-launcher-location-south-west = Bas gauche
enum-launcher-location-west = Gauche

enum-launcher-matching-normal = Normale
enum-launcher-matching-regex = Expression régulière
enum-launcher-matching-glob = Glob
enum-launcher-matching-fuzzy = Approximative
enum-launcher-matching-prefix = Préfixe

enum-launcher-sorting-levenshtein = Levenshtein
enum-launcher-sorting-fzf = FZF

enum-launcher-case-insensitive = Insensible
enum-launcher-case-smart = Intelligente
enum-launcher-case-sensitive = Sensible

enum-launcher-drun-field-name = Nom
enum-launcher-drun-field-generic = Nom générique
enum-launcher-drun-field-exec = Commande
enum-launcher-drun-field-categories = Catégories
enum-launcher-drun-field-comment = Commentaire
enum-launcher-drun-field-keywords = Mots-clés

enum-launcher-window-field-title = Titre
enum-launcher-window-field-class = Classe
enum-launcher-window-field-name = Nom
enum-launcher-window-field-role = Rôle
enum-launcher-window-field-desktop = Espace de travail

enum-launcher-file-sort-name = Nom
enum-launcher-file-sort-mtime = Modification
enum-launcher-file-sort-atime = Dernier accès
enum-launcher-file-sort-ctime = Création

settings-animations-launcher = Lanceur
    .description = Remplacement de l'animation d'entrée et de sortie du lanceur d'applications

settings-launcher-search = Panneau de recherche
    .description = Le panneau qu'ouvre lys-shell launcher sans argument

settings-launcher-search-providers = Sources
    .description = Ce que la liste recherche

settings-launcher-search-web-engine = Nom de la recherche web
    .description = Le nom du moteur sur la ligne de repli

settings-launcher-search-web-url = URL de la recherche web
    .description = L'URL, avec {"{"}query{"}"} à l'endroit de la recherche

settings-launcher-search-file-roots = Dossiers de l'index de fichiers
    .description = Dossiers parcourus par l'index de fichiers ; vide indexe le dossier personnel

settings-launcher-search-file-exclude = Exclusions de l'index de fichiers
    .description = Noms de dossiers ignorés par l'index de fichiers

settings-launcher-search-commands-dir = Dossier des commandes de script
    .description = Vide utilise launcher/commands dans le dossier de configuration

settings-launcher-search-currency-rates = Taux de change
    .description = Télécharger les taux de référence quotidiens de la Banque centrale européenne, une fois par jour au plus, pour les conversions de devises

settings-launcher-search-previews = Aperçus
    .description = Afficher le volet latéral pour les fichiers, les extraits, les fenêtres et les entrées du presse-papiers, dans le panneau de recherche et les modes fenêtre et presse-papiers

settings-launcher-search-quicklinks = Liens rapides
    .description = Liens à ouvrir par nom ou mot-clé

settings-launcher-search-snippets = Extraits
    .description = Texte à copier par nom ou mot-clé

enum-launcher-search-provider-apps = Applications
enum-launcher-search-provider-windows = Fenêtres ouvertes
enum-launcher-search-provider-files = Fichiers
enum-launcher-search-provider-calculator = Calculatrice
enum-launcher-search-provider-system = Commandes système
enum-launcher-search-provider-settings = Pages de paramètres
enum-launcher-search-provider-clipboard = Historique du presse-papiers
enum-launcher-search-provider-emoji = Émojis
enum-launcher-search-provider-characters = Caractères Unicode
enum-launcher-search-provider-vault = Coffre Bitwarden
enum-launcher-search-provider-links = Liens rapides et extraits
enum-launcher-search-provider-browser-bookmarks = Marque-pages du navigateur
enum-launcher-search-provider-browser-history = Historique du navigateur
enum-launcher-search-provider-quick-toggles = Bascules rapides
enum-launcher-search-provider-media = Commandes multimédia
enum-launcher-search-provider-capture = Captures et enregistrement
enum-launcher-search-provider-processes = Tuer un processus
enum-launcher-search-provider-scripts = Commandes de script
enum-launcher-search-provider-shell = Commandes shell
enum-launcher-search-provider-web = Recherche web

## Format placeholders

settings-launcher-drun-display-format-placeholder-name = Nom de l'application

settings-launcher-drun-display-format-placeholder-generic = Nom générique (ex. : navigateur web)

settings-launcher-drun-display-format-placeholder-exec = Commande lancée par l'entrée

settings-launcher-drun-display-format-placeholder-categories = Catégories de l'entrée de bureau

settings-launcher-drun-display-format-placeholder-comment = Commentaire de l'entrée de bureau

settings-launcher-window-format-placeholder-w = Espace de travail

settings-launcher-window-format-placeholder-c = Classe de la fenêtre

settings-launcher-window-format-placeholder-t = Titre de la fenêtre

settings-launcher-window-format-placeholder-n = Nom de la fenêtre (sa classe)

settings-launcher-window-format-placeholder-r = Rôle de la fenêtre (toujours vide)

settings-launcher-combi-display-format-placeholder-mode = Mode d'où provient la ligne

settings-launcher-combi-display-format-placeholder-text = Texte de la ligne

# Emoji

settings-launcher-emoji = Émojis
    .description = Les émojis du panneau de recherche, de la grille d'émojis et du mode émoji

settings-launcher-emoji-skin-tone = Teint
    .description = Le teint des émojis de personnes et de mains ; en choisir un dans les actions d'un émoji en fait le teint par défaut

## LauncherSkinTone variants
enum-launcher-skin-tone-none = Par défaut (jaune)
enum-launcher-skin-tone-light = Clair
enum-launcher-skin-tone-medium-light = Moyennement clair
enum-launcher-skin-tone-medium = Légèrement mat
enum-launcher-skin-tone-medium-dark = Mat
enum-launcher-skin-tone-dark = Foncé

settings-launcher-run-run-command-placeholder-cmd = La commande saisie
settings-launcher-run-shell-command-placeholder-terminal = Le terminal
settings-launcher-run-shell-command-placeholder-cmd = La commande saisie
settings-launcher-ssh-command-placeholder-terminal = Le terminal
settings-launcher-ssh-command-placeholder-ssh-client = Le client SSH
settings-launcher-ssh-command-placeholder-host = L’hôte
settings-launcher-preview-cmd-placeholder-input = Le fichier à prévisualiser
settings-launcher-preview-cmd-placeholder-output = Où écrire la miniature
settings-launcher-preview-cmd-placeholder-size = La taille de la miniature en pixels

## Quick links and snippets

settings-launcher-quicklink-name = Nom
settings-launcher-quicklink-url = Adresse
settings-launcher-quicklink-keyword = Mot-clé
settings-launcher-quicklink-icon = Icône
settings-launcher-quicklink-url-placeholder-query = Les mots tapés après le mot-clé
settings-launcher-snippet-name = Nom
settings-launcher-snippet-text = Texte
settings-launcher-snippet-keyword = Mot-clé
