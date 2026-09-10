# Documentation

| Document | What it is | Read it when |
| --- | --- | --- |
| [USAGE.md](./USAGE.md) | How to install and use the provider, resource by resource | You want to manage an Omni instance with Terraform |
| [TEST-REPORT.md](./TEST-REPORT.md) | What has been verified against a live instance, what has not, and every defect found | You need to judge whether this is safe to rely on |
| [TEST-CASES.md](./TEST-CASES.md) | The case-by-case matrix behind the report, with expected and actual results | You are running or extending the test suite |
| [index.md](./index.md) | Terraform Registry landing page | Published on the registry, not usually read here |

## Where to start

**Using it:** [USAGE.md](./USAGE.md). Section 1 if the provider is installed
from the registry, section 2 if you are running it from source.

**Deciding whether to trust it:** [TEST-REPORT.md](./TEST-REPORT.md), sections
2 and 6. Section 2 is the coverage table, section 6 is the honest list of what
is not covered and the risk of each gap.

**Changing it:** [TEST-CASES.md](./TEST-CASES.md) for the matrix, then run the
suite in `test/` or the `provider crud test` workflow before and after your
change.

## The one thing worth knowing

Seven of the defects found during testing were the same root cause: **different
Omni endpoints return different subsets and shapes of the same object.** Create
omits fields that list returns. Update omits more. Some collections come back
as objects where an array was documented.

Any new resource should assume a write response is partial, read the object
back rather than trusting what the write returned, and never let an absent
field null out something Terraform already knows.
