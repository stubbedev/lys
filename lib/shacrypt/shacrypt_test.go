package shacrypt

import (
	"errors"
	"strings"
	"testing"
)

var vectors = []struct{ setting, password, want string }{
	{"$6$saltstring", "Hello world!", "$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1"},
	{"$6$rounds=10000$saltstringsaltstring", "Hello world!", "$6$rounds=10000$saltstringsaltst$OW1/O6BYHV6BcXZu8QVeXbDWra3Oeqh0sbHbbMCVNSnCM/UrjmM0Dp8vOuZeHBy/YTBmSK6H9qs/y3RnOaw5v."},
	{"$6$rounds=5000$toolongsaltstring", "This is just a test", "$6$rounds=5000$toolongsaltstrin$lQ8jolhgVRVhY4b5pZKaysCLi0QBxGoNeKQzQ3glMhwllF7oGDZxUhx1yxdYcz/e1JSbq3y6JMxxl8audkUEm0"},
	{"$6$rounds=1400$anotherlongsaltstring", "a very much longer text to encrypt.  This one even stretches over morethan one line.", "$6$rounds=1400$anotherlongsalts$POfYwTEok97VWcjxIiSOjiykti.o/pQs.wPvMxQ6Fm7I6IoYN3CmLs66x9t0oSwbtEW7o7UmJEiDwGqd8p4ur1"},
	{"$6$rounds=77777$short", "we have a short salt string but not a short password", "$6$rounds=77777$short$WuQyW2YR.hBNpjjRhpYD/ifIw05xdfeEyQoMxIXbkvr0gge1a1x3yRULJ5CCaUeOxFmtlcGZelFl5CxtgfiAc0"},
	{"$6$rounds=123456$asaltof16chars..", "a short string", "$6$rounds=123456$asaltof16chars..$BtCwjqMJGx5hrJhZywWvt0RLE8uZ4oPwcelCjmw2kSYu.Ec6ycULevoBK25fs2xXgMNrCzIMVcgEJAstJeonj1"},
	{"$6$rounds=10$roundstoolow", "the minimum number is still observed", "$6$rounds=1000$roundstoolow$kUMsbe306n21p9R.FRkW3IGn.S9NPN0x50YhH1xhLsPuWGsUSklZt58jaTfF4ZEQpyUNGc0dqbpBYYBaHHrsX."},
}

func TestTheSpecificationsVectorsHash(t *testing.T) {
	for _, v := range vectors {
		st, err := ParseSetting(v.setting)
		if err != nil {
			t.Fatalf("%q: %v", v.setting, err)
		}
		if got := st.Hash([]byte(v.password)); got != v.want {
			t.Errorf("%q with %q:\n got %s\nwant %s", v.setting, v.password, got, v.want)
		}
	}
}

func TestAHashVerifiesItsPasswordAndNoOther(t *testing.T) {
	for _, v := range vectors {
		if ok, err := Verify([]byte(v.password), v.want); !ok || err != nil {
			t.Errorf("%q does not verify %q: %v", v.want, v.password, err)
		}
		if ok, err := Verify([]byte(v.password+"!"), v.want); ok || err != nil {
			t.Errorf("%q verifies another password: %v", v.want, err)
		}
		tampered := v.want[:len(v.want)-1] + "A"
		if strings.HasSuffix(v.want, "A") {
			tampered = v.want[:len(v.want)-1] + "B"
		}
		if ok, _ := Verify([]byte(v.password), tampered); ok {
			t.Errorf("a changed digest %q verifies", tampered)
		}
	}
}

func TestANewSettingIsARandomFullSaltAtTheDefaultRounds(t *testing.T) {
	a, b := NewSetting(), NewSetting()
	if len(a.Salt()) != MaxSalt || strings.Trim(a.Salt(), alphabet) != "" || a.Rounds() != DefaultRounds {
		t.Fatalf("a new setting is %q at %d rounds", a.Salt(), a.Rounds())
	}
	if a.Salt() == b.Salt() {
		t.Fatalf("two settings share the salt %q", a.Salt())
	}
	h := Hash([]byte("secret"))
	if !strings.HasPrefix(h, "$6$") || strings.Contains(h, "rounds=") || len(h) != len("$6$")+MaxSalt+1+86 {
		t.Fatalf("a hash reads %q", h)
	}
	if ok, err := Verify([]byte("secret"), h); !ok || err != nil {
		t.Fatalf("a new hash does not verify: %v", err)
	}
}

func TestRoundsAreClampedAndKeptInTheSetting(t *testing.T) {
	for in, want := range map[string]int{
		"$6$salt":                    DefaultRounds,
		"$6$rounds=999$salt":         MinRounds,
		"$6$rounds=1000000000$salt":  MaxRounds,
		"$6$rounds=4321$salt$digest": 4321,
	} {
		st, err := ParseSetting(in)
		if err != nil || st.Rounds() != want || st.Salt() != "salt" {
			t.Errorf("%q: %v, %d rounds, salt %q; want %d", in, err, st.Rounds(), st.Salt(), want)
		}
	}
}

func TestASettingThatIsNotOneIsRefused(t *testing.T) {
	for in, want := range map[string]error{
		"":                                    ErrPrefix,
		"$5$saltstring":                       ErrPrefix,
		"$1$saltstring":                       ErrPrefix,
		"6$saltstring":                        ErrPrefix,
		"$6$rounds=$salt":                     ErrRounds,
		"$6$rounds=12x$salt":                  ErrRounds,
		"$6$rounds=-5$salt":                   ErrRounds,
		"$6$rounds=5000":                      ErrRounds,
		"$6$rounds=99999999999999999999$salt": ErrRounds,
		"$6$sa:lt":                            ErrSalt,
		"$6$salt with space":                  ErrSalt,
	} {
		if _, err := ParseSetting(in); !errors.Is(err, want) {
			t.Errorf("%q: %v, want %v", in, err, want)
		}
		if _, err := Verify([]byte("x"), in); !errors.Is(err, want) {
			t.Errorf("verifying against %q: %v, want %v", in, err, want)
		}
	}
}
