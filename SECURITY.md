# Security policy

## Supported versions

| Version | Supported |
| --- | --- |
| 0.1.x | Yes |

This provider is pre-1.0. Fixes land on the most recent minor release; older
minors are not patched.

## Reporting a vulnerability

Please do not open a public issue for a security problem.

Report it through GitHub's private vulnerability reporting, under the
[Security tab](https://github.com/exploreomni/terraform-provider-omni/security/advisories/new)
of this repository. That opens a private advisory visible only to the
maintainers.

If you would rather use email, write to <SECURITY-CONTACT@omni.co>.

Please include:

- What the issue allows an attacker to do
- The provider version and the Omni instance version
- Steps to reproduce, with a minimal configuration if you have one
- Whether you have disclosed it anywhere else

## What to expect

- Acknowledgement within 2 business days
- An initial assessment, including whether we agree it is a vulnerability,
  within 5 business days
- For a confirmed issue, a fix or a documented mitigation in the next patch
  release, and credit in the advisory unless you ask us not to

## Scope

This policy covers the provider in this repository. A vulnerability in the Omni
platform or its REST API should go to Omni's own security contact rather than
here, and we will forward anything that arrives in the wrong place.

## Handling credentials

The provider reads `OMNI_API_TOKEN` from the environment or from the
`api_token` provider argument. Treat an Omni organization API key as you would
any other administrative credential: it can create and delete users, groups and
content.

Note that Terraform writes provider arguments and resource attributes to state
in plain text. Keep state in a backend with access control and encryption at
rest, and do not commit `terraform.tfstate` to version control.
