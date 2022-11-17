import struct
from .rpc import RPC
from .const import PORTMAP_PROGRAM, PORTMAP_VERSION, PORTMAP_PORT


class Portmap(RPC):
    program = PORTMAP_PROGRAM    # Portmap
    program_version = PORTMAP_VERSION
    port = PORTMAP_PORT

    def __init__(self, host, timeout=6000):
        super(Portmap, self).__init__(host, Portmap.port, timeout)

    def null(self):
        procedure = 0   # Null

        super(Portmap, self).request(self.program, self.program_version, procedure)

        # no exception raised
        return True

    def dump(self):
        procedure = 4   # Dump

        proto = struct.pack('!LL', self.program_version, procedure)

        portmap = super(Portmap, self).request(self.program, self.program_version, procedure, data=proto)

        rpc_map_entries = list()

        if len(portmap) <= 4:  # portmap_Value_Follows + one portmap_Map_entry
            return rpc_map_entries

        portmap_value_follows = portmap[0:4]
        portmap_map_entries = portmap[4:]

        while portmap_value_follows == b'\x00\x00\x00\x01':
            (
                program,
                version,
                protocol,
                port
            ) = struct.unpack('!LLLL', portmap_map_entries[:16])
            portmap_map_entries = portmap_map_entries[16:]

            if protocol == 0x06:
                protocol = 'tcp'
            elif protocol == 0x11:
                protocol = 'udp'
            else:
                protocol = 'unknown'.format(protocol)

            _ = {'program': program, 'version': version, 'protocol': protocol, 'port': port}
            if _ not in rpc_map_entries:
                rpc_map_entries.append(_)

            portmap_value_follows = portmap_map_entries[0:4]
            portmap_map_entries = portmap_map_entries[4:]

        return rpc_map_entries

    def getport(self, getport_program, getport_program_version, getport_protocol=6):
        # RPC
        program = 100000    # Portmap
        program_version = 2
        procedure = 3       # GetPort

        # GetPort
        getport_port = 0

        proto = struct.pack('!LLLL', getport_program, getport_program_version, getport_protocol, getport_port)

        getport = super(Portmap, self).request(program, program_version, procedure, data=proto)

        (port,) = struct.unpack('!L', getport)
        return port
