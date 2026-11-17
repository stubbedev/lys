with logs_on_failure(machine, greeter_logs):
    await_gate(machine)
    splash, _ = hand_over(machine)
    log_in(machine, splash.size)
