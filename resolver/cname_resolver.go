package resolver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
	"github.com/stgnet/blocky/config"
	"github.com/stgnet/blocky/util"
)

type CnameResolver struct {
	NextResolver
	cfg config.CnameConfig
}

// NewCnameResolver resturns a new restriction resolver
func NewCnameResolver(cfg config.CnameConfig) ChainedResolver {
	return &CnameResolver{cfg: cfg}
}

// Configuration returns the string representation of the configuration
func (cr *CnameResolver) Configuration() (result []string) {

	for k, val := range cr.cfg.Groups {
		result = append(result, fmt.Sprintf("group %s redirects to %s", k, val.Cname))
		for _, v := range val.Domains {
			result = append(result, fmt.Sprintf("domain %s", v))
		}
	}

	for key, val := range cr.cfg.ClientGroupsBlock {
		result = append(result, fmt.Sprintf("  %s = \"%s\"", key, strings.Join(val, ";")))
	}

	return
}

// Resolve requested domain and looks if it's part of any restriction
func (cr *CnameResolver) Resolve(req *Request) (*Response, error) {
	logger := withPrefix(req.Log, "cname_resolver")

	for _, question := range req.Req.Question {
		domain := util.ExtractDomain(question)
		groups := cr.groupsToCheckForClient(req)
		if len(groups) <= 0 {
			continue
		}

		if len(domain) > 0 {
			for _, g := range groups {
				for _, d := range cr.cfg.Groups[g].Domains {
					if d == domain {
						response := new(dns.Msg)
						response.SetReply(req.Req)

						rr := new(dns.CNAME)
						h := dns.RR_Header{Name: question.Name, Rrtype: question.Qtype, Class: dns.ClassINET, Ttl: customDNSTTL}
						rr.Target = cr.cfg.Groups[g].Cname
						rr.Hdr = h

						response.Answer = append(response.Answer, rr)

						logger.WithFields(logrus.Fields{
							"answer": util.AnswerToString(response.Answer),
							"domain": domain,
						}).Debugf("returning restricted dns entry")

						return &Response{Res: response, RType: CUSTOMDNS, Reason: "RESTRICTED DNS"}, nil
					}
				}
			}
		}
	}
	return cr.next.Resolve(req)
}

func (cr *CnameResolver) groupsToCheckForClient(request *Request) (groups []string) {
	// try client names
	groupsByName, found := cr.cfg.ClientGroupsBlock[request.ClientIP.String()]
	if found {
		groups = append(groups, groupsByName...)
	}

	if len(groups) == 0 {
		groups = cr.cfg.ClientGroupsBlock["default"]
	}

	sort.Strings(groups)

	return
}
