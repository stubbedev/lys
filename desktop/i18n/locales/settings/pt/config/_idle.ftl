settings-idle-enable = Política de inatividade integrada
    .description = Escurecer, bloquear, desligar os ecrãs e suspender quando inativo; desligue quando o swayidle ou o hypridle tratam da inatividade

settings-idle-dim-percent = Brilho escurecido
    .description = Percentagem do seu nível que uma retroiluminação escurecida mantém

settings-idle-suspend-action = Modo de suspensão
    .description = Como o sistema dorme quando chega a etapa de suspensão, através do logind

settings-idle-lock-before-sleep = Bloquear antes de suspender
    .description = Bloquear a sessão antes de o sistema suspender, para que acorde sempre bloqueada

settings-idle-timeouts-ac = Ligado à corrente
    .description = Tempo de inatividade como uma duração, como 90s, 5m ou 1h30m, ou never

settings-idle-timeouts-battery = Na bateria
    .description = Tempo de inatividade como uma duração, como 90s, 5m ou 1h30m, ou never

settings-idle-attention = Olhar antes de escurecer
    .description = Antes de o ecrã escurecer, ligar a câmara durante menos de um segundo, primeiro a de infravermelhos, para ver se há um rosto virado para o ecrã: um rosto adia o escurecimento. A sua luz acende-se enquanto olha. A imagem serve apenas para procurar um rosto e é descartada, nunca guardada, registada nem enviada, e nada é visto enquanto a sessão está bloqueada ou outra aplicação usa a câmara

## SleepAction variants
enum-sleep-action-auto = Suspender e depois hibernar, se possível
enum-sleep-action-suspend = Suspender
enum-sleep-action-hibernate = Hibernar
enum-sleep-action-hybrid-sleep = Suspensão híbrida
enum-sleep-action-suspend-then-hibernate = Suspender e depois hibernar

## IdleAttention variants
enum-idle-attention-off = Desligado
enum-idle-attention-postpone-dim = Adiar apenas o escurecimento
enum-idle-attention-sooner-when-absent = Desligar também o ecrã mais cedo quando não está ninguém
