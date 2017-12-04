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

# Display global

Afaik, the docs don't actually say how the "global" display object gets
identified. From reading the source, it appears it is object ID 0 on
connection.

# Id allocation:

<https://wayland.freedesktop.org/docs/html/ch04.html#sect-Protocol-Creating-Objects>
Says that clients should use the range `[1, 0xfeffffff]`, while servers
use `[0xff000000, 0xffffffff]`. 0 is supposedly reserved for null or
absent objects. However, this conflicts with:

* My reading of the way identifying the display object works, per above.
* <https://wayland.freedesktop.org/docs/html/ch04.html#sect-Protocol-Wire-Format>,
  which says that for `new_id`s, the server allocates ids below 0x10000.

I've yet to figure out what the reality is.

[1]: https://wayland.freedesktop.org/docs/html/ch04.html
