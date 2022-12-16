package main

import (
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/emersion/go-message/mail"

	imap "github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/joho/godotenv"
)

type ServerMail struct {
	user    string
	pass    string
	erro    string
	tls     string
	cliente *client.Client
}

func NewServerMail() *ServerMail {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	serverMail := &ServerMail{}
	serverMail.user = os.Getenv("USERNAME_MAIL")
	serverMail.pass = os.Getenv("PASSWORD_MAIL")
	serverMail.erro = ""
	serverMail.tls = os.Getenv("DAIL_OUTLOOK")

	return serverMail
}

func main() {

	serverMail := NewServerMail()

	serverMail.Connect()
	if serverMail.erro != "" {
		log.Fatal(serverMail.erro)
	}
	serverMail.Login()
	if serverMail.erro != "" {
		log.Fatal(serverMail.erro)
	}
	serverMail.ListUnseenMessages()
	if serverMail.erro != "" {
		log.Fatal(serverMail.erro)
	}
}

func (serverMail *ServerMail) Connect() {
	// Connect to server
	cliente, erro := client.DialTLS(serverMail.tls, nil)
	if erro != nil {
		serverMail.erro = erro.Error()
	}
	log.Println("Connected")

	serverMail.cliente = cliente

}

func (serverMail *ServerMail) Login() {
	// Login
	if erro := serverMail.cliente.Login(serverMail.user, serverMail.pass); erro != nil {
		serverMail.erro = erro.Error()
	}
	log.Println("Logged")

}

func (serverMail *ServerMail) setLabelBox(label string) *imap.MailboxStatus {
	mailbox, erro := serverMail.cliente.Select(label, true)
	if erro != nil {
		serverMail.erro = erro.Error()
	}
	return mailbox
}

func (serverMail *ServerMail) ListUnseenMessages() {

	// List mailboxes
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- serverMail.cliente.List("", "*", mailboxes)
	}()

	// set mailbox to INBOX
	serverMail.setLabelBox("INBOX")
	// criteria to search for unseen messages
	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{"\\Seen"}

	uids, err := serverMail.cliente.UidSearch(criteria)
	if err != nil {
		log.Println(err)
	}
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uids...)
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate, section.FetchItem()}
	messages := make(chan *imap.Message)
	go func() {
		if err := serverMail.cliente.UidFetch(seqSet, items, messages); err != nil {
			log.Fatal(err)
		}
	}()

	for message := range messages {

		log.Println(message.Uid)

		if message == nil {
			log.Fatal("Server didn't returned message")
		}

		r := message.GetBody(section)
		if r == nil {
			log.Fatal("Server didn't returned message body")
		}

		// Create a new mail reader
		mr, err := mail.CreateReader(r)
		if err != nil {
			log.Fatal(err)
		}

		// Print some info about the message
		header := mr.Header

		if date, err := header.Date(); err == nil {
			log.Println("Date:", date)
		}
		if from, err := header.AddressList("From"); err == nil {
			log.Println("From:", from)
		}
		if to, err := header.AddressList("To"); err == nil {
			log.Println("To:", to)
		}
		if subject, err := header.Subject(); err == nil {
			log.Println("Subject:", subject)
		}
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			// This is the message's text (can be plain-text or HTML)
			b, _ := ioutil.ReadAll(p.Body)
			log.Printf("Got text: %v", string(b))

		case *mail.AttachmentHeader:
			// This is an attachment
			filename, _ := h.Filename()
			log.Printf("Got attachment: %v\n", filename)
			// Create file with attachment name
			file, err := os.Create(filename)
			if err != nil {
				log.Fatal(err)
			}
			// using io.Copy instead of io.ReadAll to avoid insufficient memory issues
			size, err := io.Copy(file, p.Body)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Saved %v bytes into %v\n", size, filename)

		}
		// 		// MARK "SEEN" ------- STARTS HERE  ---------

		// 		seqSet.Clear()
		// 		seqSet.AddNum(message.Uid)
		// 		item := imap.FormatFlagsOp(imap.AddFlags, true)
		// 		flags := []interface{}{imap.SeenFlag}
		// 		erro := serverMail.cliente.UidStore(seqSet, item, flags, nil)
		// 		if erro != nil {
		// 			panic("error!")
		// 		}
	}
}
