# `pkg` Mocks

This folder contains mock implementations of various interfaces from
the`pkg` packages. Due to the fact that some of these
interfaces will create cycles in the dependency graph, we need to keep them
each in their own package. 