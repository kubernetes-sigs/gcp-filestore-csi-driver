import struct
import logging
from .rpc import RPC
from .pack import nfs_pro_v3Unpacker
from .const import MOUNT_PROGRAM, MOUNT_V3, MNT3_OK, MOUNTSTAT3, MNT3ERR_NOTSUPP

log = logging.getLogger(__package__)


class MountAccessError(Exception):
    pass


class Mount(RPC):
    program = MOUNT_PROGRAM
    program_version = MOUNT_V3

    def __init__(self, host, port, timeout, auth):
        super(Mount, self).__init__(host=host, port=port, timeout=timeout)
        self.path = None
        self.auth = auth

    def null(self, auth=None):
        log.debug("Mount NULL on %s" % self.host)
        super(Mount, self).request(self.program, self.program_version, 0, auth=auth if auth else self.auth)
        return {"status": MNT3_OK, "message": MOUNTSTAT3[MNT3_OK]}

    def mnt(self, path, auth=None):
        data = struct.pack('!L', len(path))
        data += path.encode()
        data += b'\x00'*((4-len(path) % 4) % 4)

        log.debug("Do mount on %s" % path)
        data = super(Mount, self).request(self.program, self.program_version, 1, data=data,
                                          auth=auth if auth else self.auth)

        unpacker = nfs_pro_v3Unpacker(data)
        res = unpacker.unpack_mountres3()
        if res["status"] == MNT3_OK:
            self.path = path
        return res

    def umnt(self, auth=None):
        if not self.path:
            log.warning("No path mounted, cannot process umount.")
            return {"status": MNT3ERR_NOTSUPP, "message": MOUNTSTAT3[MNT3ERR_NOTSUPP]}
        data = struct.pack("!L", len(self.path))
        data += self.path.encode()
        data += b"\x00" * ((4 - len(self.path) % 4) % 4)

        log.debug("Do umount on %s" % self.path)
        super(Mount, self).request(self.program, self.program_version, 3, data=data, auth=auth if auth else self.auth)

        return {"status": MNT3_OK, "message": MOUNTSTAT3[MNT3_OK]}

    def export(self):
        log.debug("Get mount export on %s" % self.host)
        export = super(Mount, self).request(self.program, self.program_version, 5)

        unpacker = nfs_pro_v3Unpacker(export)
        return unpacker.unpack_exports()
