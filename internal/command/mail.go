package command

import (
	"errors"

	"github.com/joeblew999/charter/internal/project"
)

// Email a Worker sends (Cloudflare Email Sending, through the send_email binding). To an address verified on the
// account it sends as it is. To anyone, the domain it sends from is onboarded once: Cloudflare adds the records
// that let it send for that domain (bounces, SPF, DKIM, DMARC). A subdomain keeps those apart from a domain's own
// mail: charter run mail:enable mail.example.com. It needs the Workers Paid plan.
//
//	charter run mail                     the domains that send, in the account
//	charter run mail:enable <domain>     onboard one, and show the records it has

func mail(p *project.Project, args []string) {
	if len(args) > 0 {
		check(errors.New("usage: charter run mail   (charter run mail:enable <domain> onboards one)"))
	}
	check(cloudflare(p, []string{"email", "sending", "list"}, nil))
}

func mailEnable(p *project.Project, args []string) {
	if len(args) != 1 {
		check(errors.New("usage: charter run mail:enable <domain>   (a subdomain keeps the records apart from the domain's own mail: mail.example.com)"))
	}
	check(cloudflare(p, []string{"email", "sending", "enable", args[0]}, nil))
	check(cloudflare(p, []string{"email", "sending", "dns", "get", args[0]}, nil))
	say("✓ %s sends: a Worker's send_email binding may write from an address on it, to anyone", args[0])
}
