# Nominet — .UK Zone Files Access Licence

**Program (verified 2026-11-10, official pages):** "The .UK zone files"
under the **.UK Zone Files Access Licence**. The historical name "Data
Access Agreement (DAA)" our TODO used no longer appears on nominet.uk — the
licence is the current mechanism.

- **What you get:** zone files for all .uk second-level zones (.uk, .co.uk,
  .org.uk, .me.uk, .ltd.uk, .plc.uk, .net.uk, .sch.uk) PLUS a list of all
  registered domains including suspended ones and domains without
  nameservers. Download via web or SFTP/HTTPS, at most once per 24 h.
- **Fee:** currently free (Nominet reserves the right to introduce a fee on
  30 days' notice). Licence term: one month, auto-renewing.
- **Non-member application:** email **registrars@nominet.uk** with the
  details below, then complete **MiTek ID verification**
  (https://www.nominet.uk/mitek-id-verification/) when invited; credentials
  arrive via Files.com.
- **Key terms:** no redistribution or copies without written consent, no
  high-volume automated queries against Nominet/registrar systems, secure
  storage, lawful use only.

## Draft application email (owner: fill in the [brackets] and send)

```
To: registrars@nominet.uk
Subject: Zone File Access Licence application — research use

Hello,

I would like to apply for access to the .UK zone files under the Zone File
Access Licence, as a non-member applicant. My details:

  Company:            [company name or "individual applicant"]
  Contact:            Lars Artmann
  Postal address:     [full postal address]
  Email:              [email]

I accept the Zone File Access Licence terms and conditions
(https://www.nominet.uk/policies-rules/zone-file-terms-and-conditions/).

Reason for request and intended use: I research which .uk domains run
Google Workspace or Google Cloud, detected purely from public DNS records
(MX, SPF/TXT, CNAME, A) of each domain. The zone file gives me the complete
registered-domain population to scan instead of a traffic-biased top-list
subset; results are aggregate technographics. I will not redistribute the
zone data, will not run automated queries against Nominet or registrar
systems, and will store the data securely.

I'm happy to complete identity verification and provide any further
documentation you need.

Best regards,
Lars Artmann
```

## Steps

1. Send the email above (owner action).
2. Complete MiTek ID verification when invited.
3. Receive Files.com credentials; download daily; feed the scanner via
   `--input` (the plain list of registered domains).

## Sources (fetched 2026-11-10)

- https://registrars.nominet.uk/uk-namespace/the-uk-zone-files/
- https://www.nominet.uk/policies-rules/zone-file-terms-and-conditions/
- https://www.nominet.uk/mitek-id-verification/
