package portable

import "testing"

func TestSMTPRejectsUnsafeSettings(t *testing.T) {
	for _, p := range []SetupRequest{{SMTPHost: "evil\r\n.example"}, {SMTPHost: "https://smtp.example.com"}, {SMTPHost: "smtp.example.com", SMTPUsername: "test", SMTPPassword: "test", SMTPFrom: "bad-address", Email: "admin@example.com"}} {
		if e := TestEmail(p); e == nil {
			t.Fatal("invalid SMTP input accepted")
		}
	}
}
