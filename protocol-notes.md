While the wayland protocol is [partially documented][1], there are many
things not covered by the spec. This file collects information that I've
had to determine either by reading the libwayland source, or
via experimentation.

# Opcodes

The spec indicates that the message header has a 16 bit opcode field
indicating which request/event to use, but doesn't discuss how these
opcodes are derived from the xml. Inspection of the source for the C
generator indicates that they are assigned sequentially, starting from
zero, scoped to each interface's set of events/requests. i.e., for each
interface, there is a sequence of opcodes starting from zero for that
interface's requests, and another (also starting from zero) for that
interface's events.

[1]: https://wayland.freedesktop.org/docs/html/ch04.html
