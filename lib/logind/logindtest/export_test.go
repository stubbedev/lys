package logindtest

func OnLidRead(l *Logind, read func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lidRead = read
}

func LidClosed(l *Logind) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lidClosed
}
