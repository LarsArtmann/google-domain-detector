package classify

import "strings"

// matchersFile holds the pure record-value matchers behind the classifiers:
// SPF include-set probing, the Microsoft 365 MX fleet check, and host
// normalization/suffix helpers shared by every classify rule.

// spfIncludesGoogle reports whether the TXT record is an SPF policy whose
// include set covers Google's _spf.google.com.
func spfIncludesGoogle(record string) bool {
	return spfIncludes(record, "_spf.google.com")
}

// spfIncludesMicrosoft reports whether the TXT record is an SPF policy whose
// include set covers Microsoft 365's spf.protection.outlook.com.
func spfIncludesMicrosoft(record string) bool {
	return spfIncludes(record, "spf.protection.outlook.com")
}

// spfIncludes reports whether the TXT record is an SPF policy whose include
// set covers parent (itself or any subdomain).
func spfIncludes(record, parent string) bool {
	fields := strings.Fields(record)
	if len(fields) == 0 || !strings.EqualFold(fields[0], "v=spf1") {
		return false
	}

	for _, field := range fields[1:] {
		include, ok := strings.CutPrefix(field, "include:")
		if !ok {
			continue
		}

		target := normalizeHost(include)
		if isSubdomainOf(target, parent) || target == parent {
			return true
		}
	}

	return false
}

// isMicrosoftMX reports whether the MX target is Microsoft 365's mail
// protection fleet (tenant MXs are subdomains of mail.protection.outlook.com).
func isMicrosoftMX(target string) bool {
	return isSubdomainOf(target, "mail.protection.outlook.com") || target == "mail.protection.outlook.com"
}

func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
}

func isSubdomainOf(host, parent string) bool {
	return host != parent && strings.HasSuffix(host, "."+parent)
}
