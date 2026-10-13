# Connecting a model to git

Omni can keep a model's YAML in a git repository, pushing changes made in the UI
and pulling changes made in the repo. `omni_model_git` configures that.

This is not the same as keeping *this* repository in git. Omni writes the
model's own files, so it wants a repository of its own.

## 1. Create the repository

An empty repository, separate from anything else. `omni-model-playground` is a
reasonable name.

Do not point it at `terraform-provider-omni`. Omni would write model YAML into
the provider's source tree, and a model set to `follower` would then treat that
tree as its source of truth.

## 2. Create a deploy key

On the machine where you keep credentials, not in a repository:

```bash
ssh-keygen -t ed25519 -C "omni-model-playground" -f ~/.ssh/omni_model_deploy -N ""
```

That writes two files. The public one goes to GitHub, the private one to a
GitHub Actions secret.

In the new repository: **Settings > Deploy keys > Add deploy key**. Paste the
contents of `~/.ssh/omni_model_deploy.pub` and tick **Allow write access**.
Omni pushes, so without write access it can read the model and never update it.

## 3. Add the private key as a secret

In `terraform-provider-omni`: **Settings > Secrets and variables > Actions**, a
new secret named `GIT_CREDENTIALS`, keyed the same as the model in
`var.models`:

```json
{"embed-test":{"deploy_private_key":"-----BEGIN OPENSSH PRIVATE KEY-----\nMC4C...\n-----END OPENSSH PRIVATE KEY-----\n"}}
```

The newlines have to be real `\n` escapes inside the JSON string. A key pasted
with literal line breaks is not valid JSON and the plan fails on parsing it
rather than on anything to do with git.

For `https_token` instead of a deploy key, the field is `token`.

## 4. Uncomment the git block

In `infra/models.auto.tfvars`, on the model you want connected. The commented
example is already there.

`follower` deserves a decision rather than a default. Left false, the model
stays editable in Omni and changes are pushed to the repository. Set true, the
repository becomes the only way to change the model and the Omni UI goes
read-only for it. True is what you want for a production model under review;
false is what you want while testing.

## 5. Open a pull request

The plan should show one `omni_model_git` to add. Merging applies it.

Afterwards, check the model in Omni: it should show the repository and a branch.
Making a change in the UI should produce a pull request in the new repository.

## What Terraform cannot see

The API never returns the credential, so drift on it cannot be detected. If
someone replaces the deploy key in Omni, Terraform will not notice and will not
correct it.

The credential is also stored in plain text in Terraform state, like the
warehouse passwords. Whatever backend holds state is holding this too.
