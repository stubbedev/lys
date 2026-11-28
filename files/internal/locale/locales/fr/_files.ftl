### lys-files

files-title = Fichiers
files-new-tab = Nouvel onglet
files-open = Ouvrir
files-open-in-new-tab = Ouvrir dans un nouvel onglet
files-open-with = Ouvrir avec
files-no-apps = Aucune app n’ouvre ces fichiers
files-cut = Couper
files-copy = Copier
files-paste = Coller
files-rename = Renommer
files-trash = Mettre à la corbeille
files-delete = Supprimer définitivement
files-new-folder = Nouveau dossier
files-new-file = Nouveau fichier
files-folder-name = Nom du dossier
files-file-name = Nom du fichier
files-create = Créer
files-terminal = Ouvrir un terminal ici
files-copy-path = Copier le chemin
files-properties = Propriétés
files-selection = Sélection
files-select-all = Tout sélectionner
files-select-none = Ne rien sélectionner
files-invert-selection = Inverser la sélection
files-restore = Restaurer
files-empty-trash = Vider la corbeille
files-undo = Annuler
files-cancel = Annuler
files-close = Fermer
files-close-running-title = { $count ->
    [one] Une opération est encore en cours
   *[other] { $count } opérations sont encore en cours
}
files-close-running-body = Fermer la fenêtre annule ce qui n’est pas terminé.
files-keep-open = Garder ouverte
files-stop-and-close = Annuler et fermer
files-skip = Ignorer
files-keep-both = Garder les deux
files-replace = Remplacer
files-conflict-title = { $name } se trouve déjà dans { $folder }
files-conflict-file = Remplacer le fichier existant, garder les deux ou ignorer celui-ci ?
files-conflict-folder = Remplacer le dossier existant et tout son contenu, garder les deux ou ignorer celui-ci ?
files-conflict-all = Faire de même pour chaque conflit
files-delete-title = { $count ->
    [one] Supprimer { $name } définitivement ?
   *[other] Supprimer { $count } éléments définitivement ?
}
files-delete-body = Cette action est irréversible.
files-delete-no-trash-body = Un emplacement réseau n’a pas de corbeille : les fichiers sont supprimés définitivement. Cette action est irréversible.
files-delete-confirm = Supprimer
files-empty-title = Vider la corbeille ?
files-progress-copy = Copie de { $done } sur { $of }
files-progress-move = Déplacement de { $done } sur { $of }
files-progress-delete = Suppression de { $done } sur { $of }
files-progress-trash = Mise à la corbeille de { $done } sur { $of }
files-progress-empty = Vidage de la corbeille : { $done } sur { $of }
files-progress-undo = Annulation : { $done } sur { $of }
files-copy-failed = Les fichiers n’ont pas tous pu être copiés
files-move-failed = Les fichiers n’ont pas tous pu être déplacés
files-delete-failed = Les fichiers n’ont pas tous pu être supprimés
files-trash-failed = Les fichiers n’ont pas tous pu être mis à la corbeille
files-empty-failed = La corbeille n’a pas pu être vidée
files-undo-failed = La dernière modification n’a pas pu être annulée
files-restore-failed = Les fichiers n’ont pas tous pu être restaurés
files-rename-failed = { $name } n’a pas pu être renommé
files-create-failed = { $name } n’a pas pu être créé
files-clipboard-failed = Le presse-papiers n’a pas pu être utilisé
files-paste-empty = Il n’y a aucun fichier à coller
files-nothing-to-undo = Il n’y a rien à annuler
files-missing = { $path } n’existe pas
files-open-failed = { $name } n’a pas pu être ouvert
files-sent-back = { $name } a été enregistré sur { $server }
files-send-back-failed = { $name } n’a pas pu être enregistré sur { $server }
files-conflict-remote-title = { $name } a changé sur { $server }
files-conflict-remote-body = Il a changé là-bas depuis son ouverture. Le remplacer par la version enregistrée ici, ou garder celle du serveur ?
files-keep-server = Garder celle du serveur
files-close-edits-title = { $count ->
    [one] Un fichier ouvert depuis un emplacement réseau est encore ouvert
   *[other] { $count } fichiers ouverts depuis des emplacements réseau sont encore ouverts
}
files-close-edits-body = Une fois la fenêtre fermée, les modifications qui y sont enregistrées restent sur cet ordinateur et ne sont pas renvoyées.
files-open-with-failed = { $count ->
    [one] { $app } n’a pas pu ouvrir { $name }
   *[other] { $app } n’a pas pu ouvrir les { $count } éléments
}
files-terminal-failed = Aucun terminal n’a pu être ouvert
files-properties-failed = Les propriétés n’ont pas pu être lues
files-trashed = { $count ->
    [one] { $name } a été mis à la corbeille
   *[other] { $count } éléments ont été mis à la corbeille
}
files-prop-title = { $count ->
    [one] { $name }
   *[other] { $count } éléments
}
files-prop-name = Nom
files-prop-type = Type
files-prop-location = Emplacement
files-prop-size = Taille
files-prop-contents = { $size }, { $count ->
    [one] un élément
   *[other] { $count } éléments
}
files-prop-modified = Modifié
files-prop-permissions = Permissions
files-status-items = { $count ->
    [0] Aucun élément
    [one] Un élément
   *[other] { $count } éléments
}
files-status-selected = { $total ->
    [one] Un élément sélectionné
   *[other] { $count ->
        [one] Un élément sur { $total } sélectionné
       *[other] { $count } éléments sur { $total } sélectionnés
    }
}
files-status-size = { $selection } ({ $size })
