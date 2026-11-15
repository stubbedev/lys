### lys-shell Configuration - Power Profiles Module

## Power Profiles Module Configuration

settings-modules-power-profiles-format = Formato
    .description = Stringa di formato dell'etichetta. Segnaposto: {"{{ profile }}"}

settings-modules-power-profiles-icon-power-saver = Icona risparmio energetico
    .description = Icona mostrata mentre è attivo il profilo di risparmio energetico

settings-modules-power-profiles-icon-balanced = Icona bilanciato
    .description = Icona mostrata mentre è attivo il profilo bilanciato

settings-modules-power-profiles-icon-performance = Icona prestazioni
    .description = Icona mostrata mentre è attivo il profilo prestazioni

settings-modules-power-profiles-color-power-saver = Colore risparmio energetico
    .description = Colore di icona ed etichetta mentre è attivo il profilo di risparmio energetico

settings-modules-power-profiles-color-balanced = Colore bilanciato
    .description = Colore di icona ed etichetta mentre è attivo il profilo bilanciato

settings-modules-power-profiles-color-performance = Colore prestazioni
    .description = Colore di icona ed etichetta mentre è attivo il profilo prestazioni

settings-modules-power-profiles-border-show = Mostra il bordo
    .description = Mostra un bordo attorno al pulsante

settings-modules-power-profiles-border-color = Colore del bordo
    .description = Token del colore del bordo

settings-modules-power-profiles-icon-show = Mostra l'icona
    .description = Mostra l'icona del modulo

settings-modules-power-profiles-tooltip = Mostra il suggerimento
    .description = Mostra lo stato del modulo in un suggerimento al passaggio del mouse

settings-modules-power-profiles-icon-color = Colore dell'icona
    .description = Colore di primo piano dell'icona (sostituito in base al profilo attivo)

settings-modules-power-profiles-icon-bg-color = Sfondo dell'icona
    .description = Colore di sfondo del contenitore dell'icona

settings-modules-power-profiles-label-show = Mostra l'etichetta
    .description = Mostra l'etichetta

settings-modules-power-profiles-label-color = Colore dell'etichetta
    .description = Colore del testo dell'etichetta (sostituito in base al profilo attivo)

settings-modules-power-profiles-label-max-length = Lunghezza massima dell'etichetta
    .description = Numero massimo di caratteri prima del troncamento

settings-modules-power-profiles-button-bg-color = Sfondo del pulsante
    .description = Colore di sfondo del pulsante

settings-modules-power-profiles-left-click = Clic sinistro
    .description = Azione al clic sinistro. Usa `:cycle` per il ciclo integrato dei profili, vuoto per nessuna azione, oppure un comando della shell

settings-modules-power-profiles-right-click = Clic destro
    .description = Comando della shell al clic destro

settings-modules-power-profiles-middle-click = Clic centrale
    .description = Comando della shell al clic centrale

settings-modules-power-profiles-scroll-up = Scorrimento in alto
    .description = Comando della shell allo scorrimento in alto

settings-modules-power-profiles-scroll-down = Scorrimento in basso
    .description = Comando della shell allo scorrimento in basso

settings-modules-power-profiles-auto-saver-at = Risparmio energetico automatico
    .description = Livello della batteria che attiva il risparmio energetico finché non torna l'alimentazione da rete

## Format placeholders

settings-modules-power-profiles-format-placeholder-profile = Nome del profilo attivo (power-saver, balanced, performance)

## AutoSaverLevel variants
enum-auto-saver-level-low = Basso
enum-auto-saver-level-critical = Critico
enum-auto-saver-level-never = Mai

