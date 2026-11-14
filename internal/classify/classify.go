// Package classify turns raw DNS records into evidence about which Google
// platforms a domain uses. Every classifier is a pure function over record
// values, so behavior is fully pinned by table tests without network access.
package classify

import (
	"net"
	"strings"

	"github.com/larsartmann/google-domain-detector/internal/ranges"
)

// Platform is the Google platform an evidence item points at.
type Platform string

const (
	// PlatformWorkspace is Google Workspace (Mail/Calendar/Drive).
	PlatformWorkspace Platform = "workspace"
	// PlatformGCP is Google Cloud Platform (Cloud Run, Firebase, App Engine,
	// Cloud DNS, GCS, GCP load balancers).
	PlatformGCP Platform = "gcp"
	// PlatformGoogle is generic Google usage that implies neither of the two
	// commercial platforms (e.g. Search Console verification).
	PlatformGoogle Platform = "google"
	// PlatformMicrosoft is Microsoft 365 — recorded so one scan paints the
	// full email-stack picture. It is NOT a Google platform: Microsoft
	// evidence never upgrades a Google verdict.
	PlatformMicrosoft Platform = "microsoft"
)

// Weight is how directly the record proves platform use.
type Weight string

const (
	// WeightStrong: the record can only exist when the platform is in use.
	WeightStrong Weight = "strong"
	// WeightWeak: the record is a hint, not proof (usable by non-commercial
	// Google services too).
	WeightWeak Weight = "weak"
)

// EvidenceKind enumerates the DNS fingerprints the scanner knows.
type EvidenceKind string

const (
	// EvidenceMXGoogle: MX hosts under google.com/googlemail.com — mail for
	// the domain is delivered to Google (Workspace, certain).
	EvidenceMXGoogle EvidenceKind = "mx_google"
	// EvidenceSPFGoogle: SPF policy includes _spf.google.com — the domain
	// authorizes Google servers to send its mail (Workspace, likely: free
	// Gmail "send as" also lands here).
	EvidenceSPFGoogle EvidenceKind = "spf_google"
	// EvidenceDKIMGoogle: a TXT record exists at google._domainkey.<domain>
	// — the Gmail DKIM selector is configured (Workspace, likely).
	EvidenceDKIMGoogle EvidenceKind = "dkim_google"
	// EvidenceMXMicrosoft: MX hosts under mail.protection.outlook.com — mail
	// for the domain is delivered to Microsoft 365.
	EvidenceMXMicrosoft EvidenceKind = "mx_microsoft"
	// EvidenceSPFMicrosoft: SPF policy includes spf.protection.outlook.com —
	// the domain authorizes Microsoft 365 to send its mail.
	EvidenceSPFMicrosoft EvidenceKind = "spf_microsoft"
	// EvidenceCNAMERunApp: canonical name under run.app — Cloud Run service.
	EvidenceCNAMERunApp EvidenceKind = "cname_run_app"
	// EvidenceCNAMEAppspot: canonical name under appspot.com — App Engine app.
	EvidenceCNAMEAppspot EvidenceKind = "cname_appspot"
	// EvidenceCNAMEFirebase: canonical name under web.app/firebaseapp.com —
	// Firebase Hosting site.
	EvidenceCNAMEFirebase EvidenceKind = "cname_firebase"
	// EvidenceCNAMEFunctions: canonical name under cloudfunctions.net —
	// Cloud Functions.
	EvidenceCNAMEFunctions EvidenceKind = "cname_cloudfunctions"
	// EvidenceCNAMEGoogleHosted: canonical name under googlehosted.com —
	// App Engine/Firebase custom-domain routing.
	EvidenceCNAMEGoogleHosted EvidenceKind = "cname_googlehosted"
	// EvidenceCNAMEGCS: canonical name under storage.googleapis.com — GCS
	// static-website endpoint.
	EvidenceCNAMEGCS EvidenceKind = "cname_gcs"
	// EvidenceNSCloudDNS: nameservers under googledomains.com — Google Cloud DNS.
	EvidenceNSCloudDNS EvidenceKind = "ns_clouddns"
	// EvidenceIPGCPRange: A/AAAA address inside Google's published GCP ranges
	// (cloud.json) — served by a GCP frontend.
	EvidenceIPGCPRange EvidenceKind = "ip_gcp_range"
	// EvidenceSiteVerification: google-site-verification TXT — the domain
	// owner verified with SOME Google service (generic, weak).
	EvidenceSiteVerification EvidenceKind = "google_site_verification"
)

// Evidence is one observed DNS fingerprint.
type Evidence struct {
	Kind     EvidenceKind `json:"kind"`
	Platform Platform     `json:"platform"`
	Weight   Weight       `json:"weight"`
	Host     string       `json:"host"`   // the queried host (apex or www)
	Record   string       `json:"record"` // the observed record value
}

// Confidence qualifies a verdict.
type Confidence string

const (
	ConfidenceCertain Confidence = "certain"
	ConfidenceLikely  Confidence = "likely"
	ConfidenceNone    Confidence = "none"
)

// Verdict is the platform-level conclusion drawn from all evidence.
type Verdict struct {
	Detected   bool       `json:"detected"`
	Confidence Confidence `json:"confidence"`
}

// Result is everything the scanner learned about one domain.
type Result struct {
	Domain    string     `json:"domain"`
	Evidences []Evidence `json:"evidence"`
	// Err carries the dominant lookup error (e.g. NXDOMAIN) when the domain
	// yielded no evidence at all.
	Err string `json:"error,omitempty"`
	// ErrKind classifies Err (see ErrorKind); empty when Err is empty.
	ErrKind ErrorKind `json:"error_kind,omitempty"`
	// Unscanned marks a placeholder result for an input domain a canceled
	// run never reached: no lookup ran for it. Placeholders keep reports
	// accounting for every input domain but are excluded from Counts and
	// never reach the event journal.
	Unscanned bool `json:"-"`
}

// WorkspaceVerdict: MX evidence is certain (mail is delivered to Google);
// SPF or DKIM alone is likely (free Gmail "send as" can produce both).
func (r Result) WorkspaceVerdict() Verdict {
	anyWorkspace := false

	for _, e := range r.Evidences {
		if e.Platform != PlatformWorkspace {
			continue
		}

		if e.Weight == WeightStrong { // MX: mail is delivered to Google
			return Verdict{Detected: true, Confidence: ConfidenceCertain}
		}

		anyWorkspace = true
	}

	if anyWorkspace { // SPF/DKIM: free Gmail "send as" can produce both
		return Verdict{Detected: true, Confidence: ConfidenceLikely}
	}

	return Verdict{Detected: false, Confidence: ConfidenceNone}
}

// GCPVerdict: every GCP fingerprint is strong by construction, so any GCP
// evidence yields certain.
func (r Result) GCPVerdict() Verdict {
	for _, e := range r.Evidences {
		if e.Platform == PlatformGCP && e.Weight == WeightStrong {
			return Verdict{Detected: true, Confidence: ConfidenceCertain}
		}
	}

	return Verdict{Detected: false, Confidence: ConfidenceNone}
}

// GenericGoogle reports Google evidence that implies neither Workspace nor GCP.
func (r Result) GenericGoogle() bool {
	for _, e := range r.Evidences {
		if e.Platform == PlatformGoogle {
			return true
		}
	}

	return false
}

// ClassifyMX flags mail exchangers hosted by Google.
func ClassifyMX(host string, records []*net.MX) []Evidence {
	var out []Evidence

	for _, mx := range records {
		target := normalizeHost(mx.Host)
		if isSubdomainOf(target, "google.com") || isSubdomainOf(target, "googlemail.com") {
			out = append(out, Evidence{
				Kind: EvidenceMXGoogle, Platform: PlatformWorkspace, Weight: WeightStrong,
				Host: host, Record: target,
			})
		}

		if isMicrosoftMX(target) {
			out = append(out, Evidence{
				Kind: EvidenceMXMicrosoft, Platform: PlatformMicrosoft, Weight: WeightStrong,
				Host: host, Record: target,
			})
		}
	}

	return out
}

// ClassifyTXT scans TXT records for an SPF policy that includes Google and
// for google-site-verification tokens.
func ClassifyTXT(host string, records []string) []Evidence {
	var out []Evidence

	for _, record := range records {
		if spfIncludesGoogle(record) {
			out = append(out, Evidence{
				Kind: EvidenceSPFGoogle, Platform: PlatformWorkspace, Weight: WeightWeak,
				Host: host, Record: record,
			})
		}

		if spfIncludesMicrosoft(record) {
			out = append(out, Evidence{
				Kind: EvidenceSPFMicrosoft, Platform: PlatformMicrosoft, Weight: WeightWeak,
				Host: host, Record: record,
			})
		}

		if strings.HasPrefix(record, "google-site-verification=") {
			out = append(out, Evidence{
				Kind: EvidenceSiteVerification, Platform: PlatformGoogle, Weight: WeightWeak,
				Host: host, Record: record,
			})
		}
	}

	return out
}

// ClassifyDKIM flags a present TXT record at the Gmail DKIM selector. The
// caller only queries that name when stronger Workspace evidence is absent.
func ClassifyDKIM(host string, records []string) []Evidence {
	if len(records) == 0 {
		return nil
	}

	return []Evidence{{
		Kind: EvidenceDKIMGoogle, Platform: PlatformWorkspace, Weight: WeightWeak,
		Host: host, Record: records[0],
	}}
}

// ClassifyCNAME flags canonical names pointing into Google hosting surfaces.
// An empty or self-referential target yields nothing.
func ClassifyCNAME(host, target string) []Evidence {
	target = normalizeHost(target)
	if target == "" || target == normalizeHost(host) {
		return nil
	}

	kind := EvidenceKind("")

	switch {
	case isSubdomainOf(target, "run.app"):
		kind = EvidenceCNAMERunApp
	case isSubdomainOf(target, "appspot.com"):
		kind = EvidenceCNAMEAppspot
	case isSubdomainOf(target, "web.app"), isSubdomainOf(target, "firebaseapp.com"):
		kind = EvidenceCNAMEFirebase
	case isSubdomainOf(target, "cloudfunctions.net"):
		kind = EvidenceCNAMEFunctions
	case isSubdomainOf(target, "googlehosted.com"):
		kind = EvidenceCNAMEGoogleHosted
	case isSubdomainOf(target, "storage.googleapis.com"):
		kind = EvidenceCNAMEGCS
	}

	if kind == "" {
		return nil
	}

	return []Evidence{{
		Kind: kind, Platform: PlatformGCP, Weight: WeightStrong,
		Host: host, Record: target,
	}}
}

// ClassifyNS flags nameservers operated by Google Cloud DNS.
func ClassifyNS(host string, records []*net.NS) []Evidence {
	var out []Evidence

	for _, ns := range records {
		target := normalizeHost(ns.Host)
		if isSubdomainOf(target, "googledomains.com") {
			out = append(out, Evidence{
				Kind: EvidenceNSCloudDNS, Platform: PlatformGCP, Weight: WeightStrong,
				Host: host, Record: target,
			})
		}
	}

	return out
}

// ClassifyIPs flags addresses that fall inside Google's published GCP ranges.
// A nil set yields nothing (range matching disabled).
func ClassifyIPs(host string, ips []net.IP, gcp *ranges.Set) []Evidence {
	if gcp == nil {
		return nil
	}

	var out []Evidence

	for _, ip := range ips {
		if gcp.Contains(ip) {
			out = append(out, Evidence{
				Kind: EvidenceIPGCPRange, Platform: PlatformGCP, Weight: WeightStrong,
				Host: host, Record: ip.String(),
			})
		}
	}

	return out
}
