from __future__ import absolute_import
import logging
from logging import NullHandler
from .__info__ import (__version__, __author__, __author_email__, __description__)
from .rpc import RPC
from .portmap import Portmap
from .mount import Mount
from .nfs3 import NFSv3
from .const import *

logging.getLogger(__package__).addHandler(NullHandler())

__author__ = "{} <{}>".format(__author__, __author_email__)
__version__ = __version__
__doc__ = __description__


def release_all_rpc():
    RPC.disconnect_all()


__all__ = ("Portmap", "Mount", "NFSv3", "MOUNT_PROGRAM", "MNT3_OK", "MNT3ERR_ACCES", "MNT3ERR_INVAL", "MNT3ERR_IO",
           "MNT3ERR_NAMETOOLONG", "MNT3ERR_NOENT", "MNT3ERR_NOTDIR", "MNT3ERR_NOTSUPP", "MNT3ERR_PERM",
           "MNT3ERR_SERVERFAULT", "MOUNTSTAT3", "NFSSTAT3", "NFS3_OK", "NFS3ERR_ACCES", "NFS3ERR_BAD_COOKIE",
           "NFS3ERR_BADHANDLE", "NFS3ERR_BADTYPE", "NFS3ERR_DQUOT", "NFS3ERR_EXIST", "NFS3ERR_FBIG",
           "NFS3ERR_INVAL", "NFS3ERR_IO", "NFS3ERR_ISDIR", "NFS3ERR_JUKEBOX", "NFS3ERR_MLINK", "NFS3ERR_NXIO",
           "NFS3ERR_NAMETOOLONG", "NFS3ERR_NODEV", "NFS3ERR_NOENT", "NFS3ERR_NOSPC", "NFS3ERR_NOTDIR",
           "NFS3ERR_NOT_SYNC", "NFS3ERR_NOTEMPTY", "NFS3ERR_NOTSUPP", "NFS3ERR_PERM", "NFS3ERR_REMOTE",
           "NFS3ERR_ROFS", "NFS3ERR_SERVERFAULT", "NFS3ERR_TOOSMALL", "NFS3ERR_XDEV", "NFS3ERR_STALE",
           "UNSTABLE", "DATA_SYNC", "FILE_SYNC", "DONT_CHANGE", "SET_TO_CLIENT_TIME", "SET_TO_SERVER_TIME",
           "UNCHECKED", "GUARDED", "EXCLUSIVE", "FTYPE3", "NF3REG", "NF3DIR", "NF3BLK", "NF3CHR", "NF3LNK",
           "NF3FIFO", "NF3SOCK", "NFS_PROGRAM", "NFS_V3", "MOUNT_V3", "ACCESS3_READ", "ACCESS3_LOOKUP",
           "ACCESS3_MODIFY", "ACCESS3_EXTEND", "ACCESS3_DELETE", "ACCESS3_EXECUTE", "release_all_rpc")
