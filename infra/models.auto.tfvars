# Models and their git connections.
#
# This creates the model and wires it to a repository. What the repository
# contains, the topics, views and access grants, is git's business.
#
# Note on ordering: Omni builds a connection's schema model itself, after it
# has introspected the warehouse. A model created in the same apply as its
# connection can fail with "Schema model does not exist". If that happens,
# re-run the apply. The connection will exist by then and the model will
# create. Nothing needs changing in between.

models = {

}
