### lys-files

files-title = Archivos
files-new-tab = Pestaña nueva
files-open = Abrir
files-open-in-new-tab = Abrir en una pestaña nueva
files-open-with = Abrir con
files-no-apps = Ninguna app abre estos archivos
files-cut = Cortar
files-copy = Copiar
files-paste = Pegar
files-rename = Renombrar
files-trash = Mover a la papelera
files-delete = Eliminar definitivamente
files-new-folder = Carpeta nueva
files-new-file = Archivo nuevo
files-folder-name = Nombre de la carpeta
files-file-name = Nombre del archivo
files-create = Crear
files-terminal = Abrir terminal aquí
files-copy-path = Copiar ruta
files-properties = Propiedades
files-selection = Selección
files-select-all = Seleccionar todo
files-select-none = No seleccionar nada
files-restore = Restaurar
files-empty-trash = Vaciar papelera
files-undo = Deshacer
files-cancel = Cancelar
files-close = Cerrar
files-close-running-title = { $count ->
    [one] Una operación sigue en curso
   *[other] { $count } operaciones siguen en curso
}
files-close-running-body = Al cerrar la ventana se cancela lo que no ha terminado.
files-keep-open = Mantener abierta
files-stop-and-close = Cancelar y cerrar
files-skip = Omitir
files-keep-both = Conservar ambos
files-replace = Reemplazar
files-conflict-title = { $name } ya está en { $folder }
files-conflict-file = ¿Reemplazar el archivo existente, conservar ambos u omitir este?
files-conflict-folder = ¿Reemplazar la carpeta existente con todo su contenido, conservar ambas u omitir esta?
files-conflict-all = Hacer lo mismo con cada conflicto
files-delete-title = { $count ->
    [one] ¿Eliminar { $name } definitivamente?
   *[other] ¿Eliminar { $count } elementos definitivamente?
}
files-delete-body = Esto no se puede deshacer.
files-delete-no-trash-body = Una ubicación de red no tiene papelera: los archivos se eliminan para siempre. Esto no se puede deshacer.
files-delete-confirm = Eliminar
files-empty-title = ¿Vaciar la papelera?
files-progress-copy = Copiando { $done } de { $of }
files-progress-move = Moviendo { $done } de { $of }
files-progress-delete = Eliminando { $done } de { $of }
files-progress-trash = Moviendo { $done } de { $of } a la papelera
files-progress-empty = Vaciando la papelera: { $done } de { $of }
files-progress-undo = Deshaciendo: { $done } de { $of }
files-copy-failed = No se pudieron copiar todos los archivos
files-move-failed = No se pudieron mover todos los archivos
files-delete-failed = No se pudieron eliminar todos los archivos
files-trash-failed = No se pudieron mover todos los archivos a la papelera
files-empty-failed = No se pudo vaciar la papelera
files-undo-failed = No se pudo deshacer el último cambio
files-restore-failed = No se pudieron restaurar todos los archivos
files-rename-failed = No se pudo renombrar { $name }
files-create-failed = No se pudo crear { $name }
files-clipboard-failed = No se pudo usar el portapapeles
files-paste-empty = No hay archivos para pegar
files-nothing-to-undo = No hay nada que deshacer
files-missing = { $path } no existe
files-open-failed = No se pudo abrir { $name }
files-sent-back = { $name } se guardó en { $server }
files-send-back-failed = { $name } no se pudo guardar en { $server }
files-conflict-remote-title = { $name } cambió en { $server }
files-conflict-remote-body = Cambió allí desde que se abrió. ¿Reemplazarlo con la versión guardada aquí o conservar la del servidor?
files-keep-server = Conservar la del servidor
files-close-edits-title = { $count ->
    [one] Un archivo abierto desde una ubicación de red sigue abierto
   *[other] { $count } archivos abiertos desde ubicaciones de red siguen abiertos
}
files-close-edits-body = Al cerrar la ventana, los cambios guardados en ellos se quedan en este equipo y no se envían de vuelta.
files-open-with-failed = { $count ->
    [one] { $app } no pudo abrir { $name }
   *[other] { $app } no pudo abrir los { $count } elementos
}
files-terminal-failed = No se pudo abrir ningún terminal
files-properties-failed = No se pudieron leer las propiedades
files-trashed = { $count ->
    [one] { $name } se movió a la papelera
   *[other] { $count } elementos se movieron a la papelera
}
files-prop-title = { $count ->
    [one] { $name }
   *[other] { $count } elementos
}
files-prop-name = Nombre
files-prop-type = Tipo
files-prop-location = Ubicación
files-prop-size = Tamaño
files-prop-contents = { $size }, { $count ->
    [one] un elemento
   *[other] { $count } elementos
}
files-prop-modified = Modificado
files-prop-permissions = Permisos
files-status-items = { $count ->
    [0] Ningún elemento
    [one] Un elemento
   *[other] { $count } elementos
}
files-status-selected = { $total ->
    [one] Un elemento seleccionado
   *[other] { $count ->
        [one] Uno de { $total } elementos seleccionado
       *[other] { $count } de { $total } elementos seleccionados
    }
}
files-status-size = { $selection } ({ $size })
