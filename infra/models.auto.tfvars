# Models and their git connections.
#
# This creates the model and wires it to a repository. What the repository
# contains, the topics, views and access grants, is git's business.

models = {
  "embed_metrics" = {
    kind       = "SHARED"
    connection = "embed-base"
  }
}
