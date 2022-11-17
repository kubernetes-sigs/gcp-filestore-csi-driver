# GENERAL
FALSE = 0
TRUE = 1

# PORT MAP
PORTMAP_PROGRAM = 100000
PORTMAP_VERSION = 2
PORTMAP_PORT = 111

# MOUNT RELATED
MOUNT_PROGRAM = 100005
MOUNT_V3 = 3
MNT3_OK = 0
MNT3ERR_PERM = 1
MNT3ERR_NOENT = 2
MNT3ERR_IO = 5
MNT3ERR_ACCES = 13
MNT3ERR_NOTDIR = 20
MNT3ERR_INVAL = 22
MNT3ERR_NAMETOOLONG = 63
MNT3ERR_NOTSUPP = 10004
MNT3ERR_SERVERFAULT = 10006
MOUNTSTAT3 = {
    0: 'MNT3_OK',
    1: 'MNT3ERR_PERM',
    2: 'MNT3ERR_NOENT',
    5: 'MNT3ERR_IO',
    13: 'MNT3ERR_ACCES',
    20: 'MNT3ERR_NOTDIR',
    22: 'MNT3ERR_INVAL',
    63: 'MNT3ERR_NAMETOOLONG',
    10004: 'MNT3ERR_NOTSUPP',
    10006: 'MNT3ERR_SERVERFAULT',
}

# ---------------
# NFS V3 RELATED
# ---------------
NFS_PROGRAM = 100003
NFS_V3 = 3
NFS3_MNTPATHLEN = 1024
NFS3_MNTNAMLEN = 255
# PROCEDURES
NFS3_PROCEDURE_NULL = 0
NFS3_PROCEDURE_GETATTR = 1
NFS3_PROCEDURE_SETATTR = 2
NFS3_PROCEDURE_LOOKUP = 3
NFS3_PROCEDURE_ACCESS = 4
NFS3_PROCEDURE_READLINK = 5
NFS3_PROCEDURE_READ = 6
NFS3_PROCEDURE_WRITE = 7
NFS3_PROCEDURE_CREATE = 8
NFS3_PROCEDURE_MKDIR = 9
NFS3_PROCEDURE_SYMLINK = 10
NFS3_PROCEDURE_MKNOD = 11
NFS3_PROCEDURE_REMOVE = 12
NFS3_PROCEDURE_RMDIR = 13
NFS3_PROCEDURE_RENAME = 14
NFS3_PROCEDURE_LINK = 15
NFS3_PROCEDURE_READDIR = 16
NFS3_PROCEDURE_READDIRPLUS = 17
NFS3_PROCEDURE_FSSTAT = 18
NFS3_PROCEDURE_FSINFO = 19
NFS3_PROCEDURE_PATHCONF = 20
NFS3_PROCEDURE_COMMIT = 21

NFS3_FHSIZE = 64
NFS3_COOKIEVERFSIZE = 8
NFS3_CREATEVERFSIZE = 8
NFS3_WRITEVERFSIZE = 8
NFS3_OK = 0
NFS3ERR_PERM = 1
NFS3ERR_NOENT = 2
NFS3ERR_IO = 5
NFS3ERR_NXIO = 6
NFS3ERR_ACCES = 13
NFS3ERR_EXIST = 17
NFS3ERR_XDEV = 18
NFS3ERR_NODEV = 19
NFS3ERR_NOTDIR = 20
NFS3ERR_ISDIR = 21
NFS3ERR_INVAL = 22
NFS3ERR_FBIG = 27
NFS3ERR_NOSPC = 28
NFS3ERR_ROFS = 30
NFS3ERR_MLINK = 31
NFS3ERR_NAMETOOLONG = 63
NFS3ERR_NOTEMPTY = 66
NFS3ERR_DQUOT = 69
NFS3ERR_STALE = 70
NFS3ERR_REMOTE = 71
NFS3ERR_BADHANDLE = 10001
NFS3ERR_NOT_SYNC = 10002
NFS3ERR_BAD_COOKIE = 10003
NFS3ERR_NOTSUPP = 10004
NFS3ERR_TOOSMALL = 10005
NFS3ERR_SERVERFAULT = 10006
NFS3ERR_BADTYPE = 10007
NFS3ERR_JUKEBOX = 10008
NFSSTAT3 = {
    0: 'NFS3_OK',
    1: 'NFS3ERR_PERM',
    2: 'NFS3ERR_NOENT',
    5: 'NFS3ERR_IO',
    6: 'NFS3ERR_NXIO',
    13: 'NFS3ERR_ACCES',
    17: 'NFS3ERR_EXIST',
    18: 'NFS3ERR_XDEV',
    19: 'NFS3ERR_NODEV',
    20: 'NFS3ERR_NOTDIR',
    21: 'NFS3ERR_ISDIR',
    22: 'NFS3ERR_INVAL',
    27: 'NFS3ERR_FBIG',
    28: 'NFS3ERR_NOSPC',
    30: 'NFS3ERR_ROFS',
    31: 'NFS3ERR_MLINK',
    63: 'NFS3ERR_NAMETOOLONG',
    66: 'NFS3ERR_NOTEMPTY',
    69: 'NFS3ERR_DQUOT',
    70: 'NFS3ERR_STALE',
    71: 'NFS3ERR_REMOTE',
    10001: 'NFS3ERR_BADHANDLE',
    10002: 'NFS3ERR_NOT_SYNC',
    10003: 'NFS3ERR_BAD_COOKIE',
    10004: 'NFS3ERR_NOTSUPP',
    10005: 'NFS3ERR_TOOSMALL',
    10006: 'NFS3ERR_SERVERFAULT',
    10007: 'NFS3ERR_BADTYPE',
    10008: 'NFS3ERR_JUKEBOX',
}
NF3REG = 1
NF3DIR = 2
NF3BLK = 3
NF3CHR = 4
NF3LNK = 5
NF3SOCK = 6
NF3FIFO = 7
FTYPE3 = {
    1: 'NF3REG',
    2: 'NF3DIR',
    3: 'NF3BLK',
    4: 'NF3CHR',
    5: 'NF3LNK',
    6: 'NF3SOCK',
    7: 'NF3FIFO',
}
DONT_CHANGE = 0
SET_TO_SERVER_TIME = 1
SET_TO_CLIENT_TIME = 2
time_how = {
    0: 'DONT_CHANGE',
    1: 'SET_TO_SERVER_TIME',
    2: 'SET_TO_CLIENT_TIME',
}
ACCESS3_READ = 0x0001
ACCESS3_LOOKUP = 0x0002
ACCESS3_MODIFY = 0x0004
ACCESS3_EXTEND = 0x0008
ACCESS3_DELETE = 0x0010
ACCESS3_EXECUTE = 0x0020
UNSTABLE = 0
DATA_SYNC = 1
FILE_SYNC = 2
STABLE_HOW = {
    0: 'UNSTABLE',
    1: 'DATA_SYNC',
    2: 'FILE_SYNC',
}
UNCHECKED = 0
GUARDED = 1
EXCLUSIVE = 2
CREATEMODE3 = {
    0: 'UNCHECKED',
    1: 'GUARDED',
    2: 'EXCLUSIVE',
}
FSF3_LINK = 0x0001
FSF3_SYMLINK = 0x0002
FSF3_HOMOGENEOUS = 0x0008
FSF3_CANSETTIME = 0x0010

# ---------
# NFS v4.2
# ---------
nfs_bool = {
    0: "FALSE",
    1: "TRUE",
}

# Sizes
NFS4_FHSIZE         = 128
NFS4_VERIFIER_SIZE  = 8
NFS4_OPAQUE_LIMIT   = 1024
NFS4_OTHER_SIZE     = 12
# Sizes new to NFSv4.1
NFS4_SESSIONID_SIZE = 16
NFS4_DEVICEID4_SIZE = 16
NFS4_INT64_MAX      = 0x7fffffffffffffff
NFS4_UINT64_MAX     = 0xffffffffffffffff
NFS4_INT32_MAX      = 0x7fffffff
NFS4_UINT32_MAX     = 0xffffffff

# Enum nfs_ftype4
NF4REG       = 1  # Regular File
NF4DIR       = 2  # Directory
NF4BLK       = 3  # Special File - block device
NF4CHR       = 4  # Special File - character device
NF4LNK       = 5  # Symbolic Link
NF4SOCK      = 6  # Special File - socket
NF4FIFO      = 7  # Special File - fifo
NF4ATTRDIR   = 8  # Attribute Directory
NF4NAMEDATTR = 9  # Named Attribute

nfs_ftype4 = {
    1: "NF4REG",
    2: "NF4DIR",
    3: "NF4BLK",
    4: "NF4CHR",
    5: "NF4LNK",
    6: "NF4SOCK",
    7: "NF4FIFO",
    8: "NF4ATTRDIR",
    9: "NF4NAMEDATTR",
}

# Enum nfsstat4
NFS4_OK                           = 0      # everything is okay
NFS4ERR_PERM                      = 1      # caller not privileged
NFS4ERR_NOENT                     = 2      # no such file/directory
NFS4ERR_IO                        = 5      # hard I/O error
NFS4ERR_NXIO                      = 6      # no such device
NFS4ERR_ACCESS                    = 13     # access denied
NFS4ERR_EXIST                     = 17     # file already exists
NFS4ERR_XDEV                      = 18     # different filesystems
# Unused/reserved                   19
NFS4ERR_NOTDIR                    = 20     # should be a directory
NFS4ERR_ISDIR                     = 21     # should not be directory
NFS4ERR_INVAL                     = 22     # invalid argument
NFS4ERR_FBIG                      = 27     # file exceeds server max
NFS4ERR_NOSPC                     = 28     # no space on filesystem
NFS4ERR_ROFS                      = 30     # read-only filesystem
NFS4ERR_MLINK                     = 31     # too many hard links
NFS4ERR_NAMETOOLONG               = 63     # name exceeds server max
NFS4ERR_NOTEMPTY                  = 66     # directory not empty
NFS4ERR_DQUOT                     = 69     # hard quota limit reached
NFS4ERR_STALE                     = 70     # file no longer exists
NFS4ERR_BADHANDLE                 = 10001  # Illegal filehandle
NFS4ERR_BAD_COOKIE                = 10003  # READDIR cookie is stale
NFS4ERR_NOTSUPP                   = 10004  # operation not supported
NFS4ERR_TOOSMALL                  = 10005  # response limit exceeded
NFS4ERR_SERVERFAULT               = 10006  # undefined server error
NFS4ERR_BADTYPE                   = 10007  # type invalid for CREATE
NFS4ERR_DELAY                     = 10008  # file "busy" - retry
NFS4ERR_SAME                      = 10009  # nverify says attrs same
NFS4ERR_DENIED                    = 10010  # lock unavailable
NFS4ERR_EXPIRED                   = 10011  # lock lease expired
NFS4ERR_LOCKED                    = 10012  # I/O failed due to lock
NFS4ERR_GRACE                     = 10013  # in grace period
NFS4ERR_FHEXPIRED                 = 10014  # filehandle expired
NFS4ERR_SHARE_DENIED              = 10015  # share reserve denied
NFS4ERR_WRONGSEC                  = 10016  # wrong security flavor
NFS4ERR_CLID_INUSE                = 10017  # clientid in use
# NFS4ERR_RESOURCE is not a valid error in NFSv4.1
NFS4ERR_RESOURCE                  = 10018  # resource exhaustion
NFS4ERR_MOVED                     = 10019  # filesystem relocated
NFS4ERR_NOFILEHANDLE              = 10020  # current FH is not set
NFS4ERR_MINOR_VERS_MISMATCH       = 10021  # minor vers not supp
NFS4ERR_STALE_CLIENTID            = 10022  # server has rebooted
NFS4ERR_STALE_STATEID             = 10023  # server has rebooted
NFS4ERR_OLD_STATEID               = 10024  # state is out of sync
NFS4ERR_BAD_STATEID               = 10025  # incorrect stateid
NFS4ERR_BAD_SEQID                 = 10026  # request is out of seq.
NFS4ERR_NOT_SAME                  = 10027  # verify - attrs not same
NFS4ERR_LOCK_RANGE                = 10028  # overlapping lock range
NFS4ERR_SYMLINK                   = 10029  # should be file/directory
NFS4ERR_RESTOREFH                 = 10030  # no saved filehandle
NFS4ERR_LEASE_MOVED               = 10031  # some filesystem moved
NFS4ERR_ATTRNOTSUPP               = 10032  # recommended attr not sup
NFS4ERR_NO_GRACE                  = 10033  # reclaim outside of grace
NFS4ERR_RECLAIM_BAD               = 10034  # reclaim error at server
NFS4ERR_RECLAIM_CONFLICT          = 10035  # conflict on reclaim
NFS4ERR_BADXDR                    = 10036  # XDR decode failed
NFS4ERR_LOCKS_HELD                = 10037  # file locks held at CLOSE
NFS4ERR_OPENMODE                  = 10038  # conflict in OPEN and I/O
NFS4ERR_BADOWNER                  = 10039  # owner translation bad
NFS4ERR_BADCHAR                   = 10040  # utf-8 char not supported
NFS4ERR_BADNAME                   = 10041  # name not supported
NFS4ERR_BAD_RANGE                 = 10042  # lock range not supported
NFS4ERR_LOCK_NOTSUPP              = 10043  # no atomic up/downgrade
NFS4ERR_OP_ILLEGAL                = 10044  # undefined operation
NFS4ERR_DEADLOCK                  = 10045  # file locking deadlock
NFS4ERR_FILE_OPEN                 = 10046  # open file blocks op.
NFS4ERR_ADMIN_REVOKED             = 10047  # lockowner state revoked
NFS4ERR_CB_PATH_DOWN              = 10048  # callback path down

# NFSv4.1 errors start here
NFS4ERR_BADIOMODE                 = 10049
NFS4ERR_BADLAYOUT                 = 10050
NFS4ERR_BAD_SESSION_DIGEST        = 10051
NFS4ERR_BADSESSION                = 10052
NFS4ERR_BADSLOT                   = 10053
NFS4ERR_COMPLETE_ALREADY          = 10054
NFS4ERR_CONN_NOT_BOUND_TO_SESSION = 10055
NFS4ERR_DELEG_ALREADY_WANTED      = 10056
NFS4ERR_BACK_CHAN_BUSY            = 10057  # backchan reqs outstanding
NFS4ERR_LAYOUTTRYLATER            = 10058
NFS4ERR_LAYOUTUNAVAILABLE         = 10059
NFS4ERR_NOMATCHING_LAYOUT         = 10060
NFS4ERR_RECALLCONFLICT            = 10061
NFS4ERR_UNKNOWN_LAYOUTTYPE        = 10062
NFS4ERR_SEQ_MISORDERED            = 10063  # unexpected seq.id in req
NFS4ERR_SEQUENCE_POS              = 10064  # [CB_]SEQ. op not 1st op
NFS4ERR_REQ_TOO_BIG               = 10065  # request too big
NFS4ERR_REP_TOO_BIG               = 10066  # reply too big
NFS4ERR_REP_TOO_BIG_TO_CACHE      = 10067  # rep. not all cached
NFS4ERR_RETRY_UNCACHED_REP        = 10068  # retry & rep. uncached
NFS4ERR_UNSAFE_COMPOUND           = 10069  # retry/recovery too hard
NFS4ERR_TOO_MANY_OPS              = 10070  # too many ops in [CB_]COMP
NFS4ERR_OP_NOT_IN_SESSION         = 10071  # op needs [CB_]SEQ. op
NFS4ERR_HASH_ALG_UNSUPP           = 10072  # hash alg. not supp.
# Unused/reserved                   10073
NFS4ERR_CLIENTID_BUSY             = 10074  # clientid has state
NFS4ERR_PNFS_IO_HOLE              = 10075  # IO to _SPARSE file hole
NFS4ERR_SEQ_FALSE_RETRY           = 10076  # Retry != original req.
NFS4ERR_BAD_HIGH_SLOT             = 10077  # req has bad highest_slot
NFS4ERR_DEADSESSION               = 10078  # new req sent to dead sess
NFS4ERR_ENCR_ALG_UNSUPP           = 10079  # encr alg. not supp.
NFS4ERR_PNFS_NO_LAYOUT            = 10080  # I/O without a layout
NFS4ERR_NOT_ONLY_OP               = 10081  # addl ops not allowed
NFS4ERR_WRONG_CRED                = 10082  # op done by wrong cred
NFS4ERR_WRONG_TYPE                = 10083  # op on wrong type object
NFS4ERR_DIRDELEG_UNAVAIL          = 10084  # delegation not avail.
NFS4ERR_REJECT_DELEG              = 10085  # cb rejected delegation
NFS4ERR_RETURNCONFLICT            = 10086  # layout get before return
NFS4ERR_DELEG_REVOKED             = 10087  # no return-state revoked

# NFSv4.2 errors start here
NFS4ERR_PARTNER_NOTSUPP           = 10088  # s2s not supported
NFS4ERR_PARTNER_NO_AUTH           = 10089  # s2s not authorized
NFS4ERR_UNION_NOTSUPP             = 10090  # Arm of union not supp
NFS4ERR_OFFLOAD_DENIED            = 10091  # dest not allowing copy
NFS4ERR_WRONG_LFS                 = 10092  # LFS not supported
NFS4ERR_BADLABEL                  = 10093  # incorrect label
NFS4ERR_OFFLOAD_NO_REQS           = 10094  # dest not meeting reqs

nfsstat4 = {
        0: "NFS4_OK",
        1: "NFS4ERR_PERM",
        2: "NFS4ERR_NOENT",
        5: "NFS4ERR_IO",
        6: "NFS4ERR_NXIO",
       13: "NFS4ERR_ACCESS",
       17: "NFS4ERR_EXIST",
       18: "NFS4ERR_XDEV",
       20: "NFS4ERR_NOTDIR",
       21: "NFS4ERR_ISDIR",
       22: "NFS4ERR_INVAL",
       27: "NFS4ERR_FBIG",
       28: "NFS4ERR_NOSPC",
       30: "NFS4ERR_ROFS",
       31: "NFS4ERR_MLINK",
       63: "NFS4ERR_NAMETOOLONG",
       66: "NFS4ERR_NOTEMPTY",
       69: "NFS4ERR_DQUOT",
       70: "NFS4ERR_STALE",
    10001: "NFS4ERR_BADHANDLE",
    10003: "NFS4ERR_BAD_COOKIE",
    10004: "NFS4ERR_NOTSUPP",
    10005: "NFS4ERR_TOOSMALL",
    10006: "NFS4ERR_SERVERFAULT",
    10007: "NFS4ERR_BADTYPE",
    10008: "NFS4ERR_DELAY",
    10009: "NFS4ERR_SAME",
    10010: "NFS4ERR_DENIED",
    10011: "NFS4ERR_EXPIRED",
    10012: "NFS4ERR_LOCKED",
    10013: "NFS4ERR_GRACE",
    10014: "NFS4ERR_FHEXPIRED",
    10015: "NFS4ERR_SHARE_DENIED",
    10016: "NFS4ERR_WRONGSEC",
    10017: "NFS4ERR_CLID_INUSE",
    10018: "NFS4ERR_RESOURCE",
    10019: "NFS4ERR_MOVED",
    10020: "NFS4ERR_NOFILEHANDLE",
    10021: "NFS4ERR_MINOR_VERS_MISMATCH",
    10022: "NFS4ERR_STALE_CLIENTID",
    10023: "NFS4ERR_STALE_STATEID",
    10024: "NFS4ERR_OLD_STATEID",
    10025: "NFS4ERR_BAD_STATEID",
    10026: "NFS4ERR_BAD_SEQID",
    10027: "NFS4ERR_NOT_SAME",
    10028: "NFS4ERR_LOCK_RANGE",
    10029: "NFS4ERR_SYMLINK",
    10030: "NFS4ERR_RESTOREFH",
    10031: "NFS4ERR_LEASE_MOVED",
    10032: "NFS4ERR_ATTRNOTSUPP",
    10033: "NFS4ERR_NO_GRACE",
    10034: "NFS4ERR_RECLAIM_BAD",
    10035: "NFS4ERR_RECLAIM_CONFLICT",
    10036: "NFS4ERR_BADXDR",
    10037: "NFS4ERR_LOCKS_HELD",
    10038: "NFS4ERR_OPENMODE",
    10039: "NFS4ERR_BADOWNER",
    10040: "NFS4ERR_BADCHAR",
    10041: "NFS4ERR_BADNAME",
    10042: "NFS4ERR_BAD_RANGE",
    10043: "NFS4ERR_LOCK_NOTSUPP",
    10044: "NFS4ERR_OP_ILLEGAL",
    10045: "NFS4ERR_DEADLOCK",
    10046: "NFS4ERR_FILE_OPEN",
    10047: "NFS4ERR_ADMIN_REVOKED",
    10048: "NFS4ERR_CB_PATH_DOWN",
    10049: "NFS4ERR_BADIOMODE",
    10050: "NFS4ERR_BADLAYOUT",
    10051: "NFS4ERR_BAD_SESSION_DIGEST",
    10052: "NFS4ERR_BADSESSION",
    10053: "NFS4ERR_BADSLOT",
    10054: "NFS4ERR_COMPLETE_ALREADY",
    10055: "NFS4ERR_CONN_NOT_BOUND_TO_SESSION",
    10056: "NFS4ERR_DELEG_ALREADY_WANTED",
    10057: "NFS4ERR_BACK_CHAN_BUSY",
    10058: "NFS4ERR_LAYOUTTRYLATER",
    10059: "NFS4ERR_LAYOUTUNAVAILABLE",
    10060: "NFS4ERR_NOMATCHING_LAYOUT",
    10061: "NFS4ERR_RECALLCONFLICT",
    10062: "NFS4ERR_UNKNOWN_LAYOUTTYPE",
    10063: "NFS4ERR_SEQ_MISORDERED",
    10064: "NFS4ERR_SEQUENCE_POS",
    10065: "NFS4ERR_REQ_TOO_BIG",
    10066: "NFS4ERR_REP_TOO_BIG",
    10067: "NFS4ERR_REP_TOO_BIG_TO_CACHE",
    10068: "NFS4ERR_RETRY_UNCACHED_REP",
    10069: "NFS4ERR_UNSAFE_COMPOUND",
    10070: "NFS4ERR_TOO_MANY_OPS",
    10071: "NFS4ERR_OP_NOT_IN_SESSION",
    10072: "NFS4ERR_HASH_ALG_UNSUPP",
    10074: "NFS4ERR_CLIENTID_BUSY",
    10075: "NFS4ERR_PNFS_IO_HOLE",
    10076: "NFS4ERR_SEQ_FALSE_RETRY",
    10077: "NFS4ERR_BAD_HIGH_SLOT",
    10078: "NFS4ERR_DEADSESSION",
    10079: "NFS4ERR_ENCR_ALG_UNSUPP",
    10080: "NFS4ERR_PNFS_NO_LAYOUT",
    10081: "NFS4ERR_NOT_ONLY_OP",
    10082: "NFS4ERR_WRONG_CRED",
    10083: "NFS4ERR_WRONG_TYPE",
    10084: "NFS4ERR_DIRDELEG_UNAVAIL",
    10085: "NFS4ERR_REJECT_DELEG",
    10086: "NFS4ERR_RETURNCONFLICT",
    10087: "NFS4ERR_DELEG_REVOKED",
    10088: "NFS4ERR_PARTNER_NOTSUPP",
    10089: "NFS4ERR_PARTNER_NO_AUTH",
    10090: "NFS4ERR_UNION_NOTSUPP",
    10091: "NFS4ERR_OFFLOAD_DENIED",
    10092: "NFS4ERR_WRONG_LFS",
    10093: "NFS4ERR_BADLABEL",
    10094: "NFS4ERR_OFFLOAD_NO_REQS",
}

# Enum time_how4
SET_TO_SERVER_TIME4 = 0
SET_TO_CLIENT_TIME4 = 1

time_how4 = {
    0: "SET_TO_SERVER_TIME4",
    1: "SET_TO_CLIENT_TIME4",
}

# Various Access Control Entry definitions
#
# Mask that indicates which Access Control Entries are supported.
# Values for the fattr4_aclsupport attribute.
ACL4_SUPPORT_ALLOW_ACL          = 0x00000001
ACL4_SUPPORT_DENY_ACL           = 0x00000002
ACL4_SUPPORT_AUDIT_ACL          = 0x00000004
ACL4_SUPPORT_ALARM_ACL          = 0x00000008

# acetype4 values, others can be added as needed.
ACE4_ACCESS_ALLOWED_ACE_TYPE    = 0x00000000
ACE4_ACCESS_DENIED_ACE_TYPE     = 0x00000001
ACE4_SYSTEM_AUDIT_ACE_TYPE      = 0x00000002
ACE4_SYSTEM_ALARM_ACE_TYPE      = 0x00000003

# ACE flag values
ACE4_FILE_INHERIT_ACE           = 0x00000001
ACE4_DIRECTORY_INHERIT_ACE      = 0x00000002
ACE4_NO_PROPAGATE_INHERIT_ACE   = 0x00000004
ACE4_INHERIT_ONLY_ACE           = 0x00000008
ACE4_SUCCESSFUL_ACCESS_ACE_FLAG = 0x00000010
ACE4_FAILED_ACCESS_ACE_FLAG     = 0x00000020
ACE4_IDENTIFIER_GROUP           = 0x00000040
ACE4_INHERITED_ACE              = 0x00000080  # New to NFSv4.1

# ACE mask values
ACE4_READ_DATA                  = 0x00000001
ACE4_LIST_DIRECTORY             = 0x00000001
ACE4_WRITE_DATA                 = 0x00000002
ACE4_ADD_FILE                   = 0x00000002
ACE4_APPEND_DATA                = 0x00000004
ACE4_ADD_SUBDIRECTORY           = 0x00000004
ACE4_READ_NAMED_ATTRS           = 0x00000008
ACE4_WRITE_NAMED_ATTRS          = 0x00000010
ACE4_EXECUTE                    = 0x00000020
ACE4_DELETE_CHILD               = 0x00000040
ACE4_READ_ATTRIBUTES            = 0x00000080
ACE4_WRITE_ATTRIBUTES           = 0x00000100
ACE4_WRITE_RETENTION            = 0x00000200  # New to NFSv4.1
ACE4_WRITE_RETENTION_HOLD       = 0x00000400  # New to NFSv4.1
ACE4_DELETE                     = 0x00010000
ACE4_READ_ACL                   = 0x00020000
ACE4_WRITE_ACL                  = 0x00040000
ACE4_WRITE_OWNER                = 0x00080000
ACE4_SYNCHRONIZE                = 0x00100000

# ACE4_GENERIC_READ -- defined as combination of
#      ACE4_READ_ACL |
#      ACE4_READ_DATA |
#      ACE4_READ_ATTRIBUTES |
#      ACE4_SYNCHRONIZE
ACE4_GENERIC_READ               = 0x00120081

# ACE4_GENERIC_WRITE -- defined as combination of
#      ACE4_READ_ACL |
#      ACE4_WRITE_DATA |
#      ACE4_WRITE_ATTRIBUTES |
#      ACE4_WRITE_ACL |
#      ACE4_APPEND_DATA |
#      ACE4_SYNCHRONIZE
ACE4_GENERIC_WRITE              = 0x00160106

# ACE4_GENERIC_EXECUTE -- defined as combination of
#      ACE4_READ_ACL
#      ACE4_READ_ATTRIBUTES
#      ACE4_EXECUTE
#      ACE4_SYNCHRONIZE
ACE4_GENERIC_EXECUTE            = 0x001200A0

# ACL flag values new to NFSv4.1
ACL4_AUTO_INHERIT = 0x00000001
ACL4_PROTECTED    = 0x00000002
ACL4_DEFAULTED    = 0x00000004

# Field definitions for the fattr4_mode attribute
# and fattr4_mode_set_masked attributes.
MODE4_SUID = 0x800  # set user id on execution
MODE4_SGID = 0x400  # set group id on execution
MODE4_SVTX = 0x200  # save text even after use
MODE4_RUSR = 0x100  # read permission: owner
MODE4_WUSR = 0x080  # write permission: owner
MODE4_XUSR = 0x040  # execute permission: owner
MODE4_RGRP = 0x020  # read permission: group
MODE4_WGRP = 0x010  # write permission: group
MODE4_XGRP = 0x008  # execute permission: group
MODE4_ROTH = 0x004  # read permission: other
MODE4_WOTH = 0x002  # write permission: other
MODE4_XOTH = 0x001  # execute permission: other

# Enum stable_how4
UNSTABLE4  = 0
DATA_SYNC4 = 1
FILE_SYNC4 = 2

stable_how4 = {
    0: "UNSTABLE4",
    1: "DATA_SYNC4",
    2: "FILE_SYNC4",
}

# Values for fattr4_fh_expire_type
FH4_PERSISTENT         = 0x00000000
FH4_NOEXPIRE_WITH_OPEN = 0x00000001
FH4_VOLATILE_ANY       = 0x00000002
FH4_VOL_MIGRATION      = 0x00000004
FH4_VOL_RENAME         = 0x00000008

# Enum layouttype4
LAYOUT4_NFSV4_1_FILES = 0x1
LAYOUT4_OSD2_OBJECTS  = 0x2
LAYOUT4_BLOCK_VOLUME  = 0x3
LAYOUT4_FLEX_FILES    = 0x4

layouttype4 = {
    0x1: "LAYOUT4_NFSV4_1_FILES",
    0x2: "LAYOUT4_OSD2_OBJECTS",
    0x3: "LAYOUT4_BLOCK_VOLUME",
    0x4: "LAYOUT4_FLEX_FILES",
}

NFL4_UFLG_MASK                  = 0x0000003F
NFL4_UFLG_DENSE                 = 0x00000001
NFL4_UFLG_COMMIT_THRU_MDS       = 0x00000002
NFL42_UFLG_IO_ADVISE_THRU_MDS   = 0x00000004
NFL4_UFLG_STRIPE_UNIT_SIZE_MASK = 0xFFFFFFC0

# Enum filelayout_hint_care4
NFLH4_CARE_DENSE              = NFL4_UFLG_DENSE
NFLH4_CARE_COMMIT_THRU_MDS    = NFL4_UFLG_COMMIT_THRU_MDS
NFL42_CARE_IO_ADVISE_THRU_MDS = NFL42_UFLG_IO_ADVISE_THRU_MDS
NFLH4_CARE_STRIPE_UNIT_SIZE   = 0x00000040
NFLH4_CARE_STRIPE_COUNT       = 0x00000080

filelayout_hint_care4 = {
                  NFL4_UFLG_DENSE : "NFLH4_CARE_DENSE",
        NFL4_UFLG_COMMIT_THRU_MDS : "NFLH4_CARE_COMMIT_THRU_MDS",
    NFL42_UFLG_IO_ADVISE_THRU_MDS : "NFL42_CARE_IO_ADVISE_THRU_MDS",
                       0x00000040 : "NFLH4_CARE_STRIPE_UNIT_SIZE",
                       0x00000080 : "NFLH4_CARE_STRIPE_COUNT",
}

# NFSv4.x flex files layout definitions

FF_FLAGS_NO_LAYOUTCOMMIT = 1

# Enum ff_cb_recall_any_mask
FF_RCA4_TYPE_MASK_READ = -2
FF_RCA4_TYPE_MASK_RW   = -1

ff_cb_recall_any_mask = {
    -2: "FF_RCA4_TYPE_MASK_READ",
    -1: "FF_RCA4_TYPE_MASK_RW",
}

# Enum layoutiomode4
LAYOUTIOMODE4_READ = 1
LAYOUTIOMODE4_RW   = 2
LAYOUTIOMODE4_ANY  = 3

layoutiomode4 = {
    1: "LAYOUTIOMODE4_READ",
    2: "LAYOUTIOMODE4_RW",
    3: "LAYOUTIOMODE4_ANY",
}
# Constants used for LAYOUTRETURN and CB_LAYOUTRECALL
LAYOUT4_RET_REC_FILE = 1
LAYOUT4_RET_REC_FSID = 2
LAYOUT4_RET_REC_ALL  = 3

# Enum layoutreturn_type4
LAYOUTRETURN4_FILE = LAYOUT4_RET_REC_FILE
LAYOUTRETURN4_FSID = LAYOUT4_RET_REC_FSID
LAYOUTRETURN4_ALL  = LAYOUT4_RET_REC_ALL

layoutreturn_type4 = {
    LAYOUT4_RET_REC_FILE : "LAYOUTRETURN4_FILE",
    LAYOUT4_RET_REC_FSID : "LAYOUTRETURN4_FSID",
     LAYOUT4_RET_REC_ALL : "LAYOUTRETURN4_ALL",
}

# Enum fs4_status_type
STATUS4_FIXED     = 1
STATUS4_UPDATED   = 2
STATUS4_VERSIONED = 3
STATUS4_WRITABLE  = 4
STATUS4_REFERRAL  = 5

fs4_status_type = {
    1: "STATUS4_FIXED",
    2: "STATUS4_UPDATED",
    3: "STATUS4_VERSIONED",
    4: "STATUS4_WRITABLE",
    5: "STATUS4_REFERRAL",
}

TH4_READ_SIZE    = 0
TH4_WRITE_SIZE   = 1
TH4_READ_IOSIZE  = 2
TH4_WRITE_IOSIZE = 3

RET4_DURATION_INFINITE = 0xffffffffffffffff

# Byte indices of items within
# fls_info: flag fields, class numbers,
# bytes indicating ranks and orders.
FSLI4BX_GFLAGS     = 0
FSLI4BX_TFLAGS     = 1
FSLI4BX_CLSIMUL    = 2
FSLI4BX_CLHANDLE   = 3
FSLI4BX_CLFILEID   = 4
FSLI4BX_CLWRITEVER = 5
FSLI4BX_CLCHANGE   = 6
FSLI4BX_CLREADDIR  = 7
FSLI4BX_READRANK   = 8
FSLI4BX_WRITERANK  = 9
FSLI4BX_READORDER  = 10
FSLI4BX_WRITEORDER = 11

# Bits defined within the general flag byte.
FSLI4GF_WRITABLE   = 0x01
FSLI4GF_CUR_REQ    = 0x02
FSLI4GF_ABSENT     = 0x04
FSLI4GF_GOING      = 0x08
FSLI4GF_SPLIT      = 0x10

# Bits defined within the transport flag byte.
FSLI4TF_RDMA       = 0x01

# Flag bits in fli_flags.
FSLI4IF_VAR_SUB    = 0x00000001
# Constants for fs_charset_cap4
FSCHARSET_CAP4_CONTAINS_NON_UTF8 = 0x1
FSCHARSET_CAP4_ALLOWS_ONLY_UTF8  = 0x2

# Enum netloc_type4
NL4_NAME    = 1
NL4_URL     = 2
NL4_NETADDR = 3

netloc_type4 = {
    1: "NL4_NAME",
    2: "NL4_URL",
    3: "NL4_NETADDR",
}

# Enum change_attr_type4
NFS4_CHANGE_TYPE_IS_MONOTONIC_INCR         = 0
NFS4_CHANGE_TYPE_IS_VERSION_COUNTER        = 1
NFS4_CHANGE_TYPE_IS_VERSION_COUNTER_NOPNFS = 2
NFS4_CHANGE_TYPE_IS_TIME_METADATA          = 3
NFS4_CHANGE_TYPE_IS_UNDEFINED              = 4

change_attr_type4 = {
    0: "NFS4_CHANGE_TYPE_IS_MONOTONIC_INCR",
    1: "NFS4_CHANGE_TYPE_IS_VERSION_COUNTER",
    2: "NFS4_CHANGE_TYPE_IS_VERSION_COUNTER_NOPNFS",
    3: "NFS4_CHANGE_TYPE_IS_TIME_METADATA",
    4: "NFS4_CHANGE_TYPE_IS_UNDEFINED",
}

# Enum nfs_fattr4

# Mandatory Attributes
FATTR4_SUPPORTED_ATTRS    = 0
FATTR4_TYPE               = 1
FATTR4_FH_EXPIRE_TYPE     = 2
FATTR4_CHANGE             = 3
FATTR4_SIZE               = 4
FATTR4_LINK_SUPPORT       = 5
FATTR4_SYMLINK_SUPPORT    = 6
FATTR4_NAMED_ATTR         = 7
FATTR4_FSID               = 8
FATTR4_UNIQUE_HANDLES     = 9
FATTR4_LEASE_TIME         = 10
FATTR4_RDATTR_ERROR       = 11
FATTR4_FILEHANDLE         = 19
FATTR4_SUPPATTR_EXCLCREAT = 75  # New to NFSv4.1

# Recommended Attributes
FATTR4_ACL                = 12
FATTR4_ACLSUPPORT         = 13
FATTR4_ARCHIVE            = 14
FATTR4_CANSETTIME         = 15
FATTR4_CASE_INSENSITIVE   = 16
FATTR4_CASE_PRESERVING    = 17
FATTR4_CHOWN_RESTRICTED   = 18
FATTR4_FILEID             = 20
FATTR4_FILES_AVAIL        = 21
FATTR4_FILES_FREE         = 22
FATTR4_FILES_TOTAL        = 23
FATTR4_FS_LOCATIONS       = 24
FATTR4_HIDDEN             = 25
FATTR4_HOMOGENEOUS        = 26
FATTR4_MAXFILESIZE        = 27
FATTR4_MAXLINK            = 28
FATTR4_MAXNAME            = 29
FATTR4_MAXREAD            = 30
FATTR4_MAXWRITE           = 31
FATTR4_MIMETYPE           = 32
FATTR4_MODE               = 33
FATTR4_NO_TRUNC           = 34
FATTR4_NUMLINKS           = 35
FATTR4_OWNER              = 36
FATTR4_OWNER_GROUP        = 37
FATTR4_QUOTA_AVAIL_HARD   = 38
FATTR4_QUOTA_AVAIL_SOFT   = 39
FATTR4_QUOTA_USED         = 40
FATTR4_RAWDEV             = 41
FATTR4_SPACE_AVAIL        = 42
FATTR4_SPACE_FREE         = 43
FATTR4_SPACE_TOTAL        = 44
FATTR4_SPACE_USED         = 45
FATTR4_SYSTEM             = 46
FATTR4_TIME_ACCESS        = 47
FATTR4_TIME_ACCESS_SET    = 48
FATTR4_TIME_BACKUP        = 49
FATTR4_TIME_CREATE        = 50
FATTR4_TIME_DELTA         = 51
FATTR4_TIME_METADATA      = 52
FATTR4_TIME_MODIFY        = 53
FATTR4_TIME_MODIFY_SET    = 54
FATTR4_MOUNTED_ON_FILEID  = 55

# New to NFSv4.1
FATTR4_DIR_NOTIF_DELAY    = 56
FATTR4_DIRENT_NOTIF_DELAY = 57
FATTR4_DACL               = 58
FATTR4_SACL               = 59
FATTR4_CHANGE_POLICY      = 60
FATTR4_FS_STATUS          = 61
FATTR4_FS_LAYOUT_TYPES    = 62
FATTR4_LAYOUT_HINT        = 63
FATTR4_LAYOUT_TYPES       = 64
FATTR4_LAYOUT_BLKSIZE     = 65
FATTR4_LAYOUT_ALIGNMENT   = 66
FATTR4_FS_LOCATIONS_INFO  = 67
FATTR4_MDSTHRESHOLD       = 68
FATTR4_RETENTION_GET      = 69
FATTR4_RETENTION_SET      = 70
FATTR4_RETENTEVT_GET      = 71
FATTR4_RETENTEVT_SET      = 72
FATTR4_RETENTION_HOLD     = 73
FATTR4_MODE_SET_MASKED    = 74
FATTR4_FS_CHARSET_CAP     = 76

# New to NFSv4.2
FATTR4_CLONE_BLKSIZE      = 77
FATTR4_SPACE_FREED        = 78
FATTR4_CHANGE_ATTR_TYPE   = 79
FATTR4_SEC_LABEL          = 80

nfs_fattr4 = {
     0: "FATTR4_SUPPORTED_ATTRS",
     1: "FATTR4_TYPE",
     2: "FATTR4_FH_EXPIRE_TYPE",
     3: "FATTR4_CHANGE",
     4: "FATTR4_SIZE",
     5: "FATTR4_LINK_SUPPORT",
     6: "FATTR4_SYMLINK_SUPPORT",
     7: "FATTR4_NAMED_ATTR",
     8: "FATTR4_FSID",
     9: "FATTR4_UNIQUE_HANDLES",
    10: "FATTR4_LEASE_TIME",
    11: "FATTR4_RDATTR_ERROR",
    12: "FATTR4_ACL",
    13: "FATTR4_ACLSUPPORT",
    14: "FATTR4_ARCHIVE",
    15: "FATTR4_CANSETTIME",
    16: "FATTR4_CASE_INSENSITIVE",
    17: "FATTR4_CASE_PRESERVING",
    18: "FATTR4_CHOWN_RESTRICTED",
    19: "FATTR4_FILEHANDLE",
    20: "FATTR4_FILEID",
    21: "FATTR4_FILES_AVAIL",
    22: "FATTR4_FILES_FREE",
    23: "FATTR4_FILES_TOTAL",
    24: "FATTR4_FS_LOCATIONS",
    25: "FATTR4_HIDDEN",
    26: "FATTR4_HOMOGENEOUS",
    27: "FATTR4_MAXFILESIZE",
    28: "FATTR4_MAXLINK",
    29: "FATTR4_MAXNAME",
    30: "FATTR4_MAXREAD",
    31: "FATTR4_MAXWRITE",
    32: "FATTR4_MIMETYPE",
    33: "FATTR4_MODE",
    34: "FATTR4_NO_TRUNC",
    35: "FATTR4_NUMLINKS",
    36: "FATTR4_OWNER",
    37: "FATTR4_OWNER_GROUP",
    38: "FATTR4_QUOTA_AVAIL_HARD",
    39: "FATTR4_QUOTA_AVAIL_SOFT",
    40: "FATTR4_QUOTA_USED",
    41: "FATTR4_RAWDEV",
    42: "FATTR4_SPACE_AVAIL",
    43: "FATTR4_SPACE_FREE",
    44: "FATTR4_SPACE_TOTAL",
    45: "FATTR4_SPACE_USED",
    46: "FATTR4_SYSTEM",
    47: "FATTR4_TIME_ACCESS",
    48: "FATTR4_TIME_ACCESS_SET",
    49: "FATTR4_TIME_BACKUP",
    50: "FATTR4_TIME_CREATE",
    51: "FATTR4_TIME_DELTA",
    52: "FATTR4_TIME_METADATA",
    53: "FATTR4_TIME_MODIFY",
    54: "FATTR4_TIME_MODIFY_SET",
    55: "FATTR4_MOUNTED_ON_FILEID",
    56: "FATTR4_DIR_NOTIF_DELAY",
    57: "FATTR4_DIRENT_NOTIF_DELAY",
    58: "FATTR4_DACL",
    59: "FATTR4_SACL",
    60: "FATTR4_CHANGE_POLICY",
    61: "FATTR4_FS_STATUS",
    62: "FATTR4_FS_LAYOUT_TYPES",
    63: "FATTR4_LAYOUT_HINT",
    64: "FATTR4_LAYOUT_TYPES",
    65: "FATTR4_LAYOUT_BLKSIZE",
    66: "FATTR4_LAYOUT_ALIGNMENT",
    67: "FATTR4_FS_LOCATIONS_INFO",
    68: "FATTR4_MDSTHRESHOLD",
    69: "FATTR4_RETENTION_GET",
    70: "FATTR4_RETENTION_SET",
    71: "FATTR4_RETENTEVT_GET",
    72: "FATTR4_RETENTEVT_SET",
    73: "FATTR4_RETENTION_HOLD",
    74: "FATTR4_MODE_SET_MASKED",
    75: "FATTR4_SUPPATTR_EXCLCREAT",
    76: "FATTR4_FS_CHARSET_CAP",
    77: "FATTR4_CLONE_BLKSIZE",
    78: "FATTR4_SPACE_FREED",
    79: "FATTR4_CHANGE_ATTR_TYPE",
    80: "FATTR4_SEC_LABEL",
}

# Enum ssv_subkey4
SSV4_SUBKEY_MIC_I2T  = 1
SSV4_SUBKEY_MIC_T2I  = 2
SSV4_SUBKEY_SEAL_I2T = 3
SSV4_SUBKEY_SEAL_T2I = 4

ssv_subkey4 = {
    1: "SSV4_SUBKEY_MIC_I2T",
    2: "SSV4_SUBKEY_MIC_T2I",
    3: "SSV4_SUBKEY_SEAL_I2T",
    4: "SSV4_SUBKEY_SEAL_T2I",
}

# Enum nfs_opnum4
OP_ACCESS               = 3
OP_CLOSE                = 4
OP_COMMIT               = 5
OP_CREATE               = 6
OP_DELEGPURGE           = 7
OP_DELEGRETURN          = 8
OP_GETATTR              = 9
OP_GETFH                = 10
OP_LINK                 = 11
OP_LOCK                 = 12
OP_LOCKT                = 13
OP_LOCKU                = 14
OP_LOOKUP               = 15
OP_LOOKUPP              = 16
OP_NVERIFY              = 17
OP_OPEN                 = 18
OP_OPENATTR             = 19
OP_OPEN_CONFIRM         = 20     # Mandatory not-to-implement in NFSv4.1
OP_OPEN_DOWNGRADE       = 21
OP_PUTFH                = 22
OP_PUTPUBFH             = 23
OP_PUTROOTFH            = 24
OP_READ                 = 25
OP_READDIR              = 26
OP_READLINK             = 27
OP_REMOVE               = 28
OP_RENAME               = 29
OP_RENEW                = 30     # Mandatory not-to-implement in NFSv4.1
OP_RESTOREFH            = 31
OP_SAVEFH               = 32
OP_SECINFO              = 33
OP_SETATTR              = 34
OP_SETCLIENTID          = 35     # Mandatory not-to-implement in NFSv4.1
OP_SETCLIENTID_CONFIRM  = 36     # Mandatory not-to-implement in NFSv4.1
OP_VERIFY               = 37
OP_WRITE                = 38
OP_RELEASE_LOCKOWNER    = 39     # Mandatory not-to-implement in NFSv4.1
# New operations for NFSv4.1
OP_BACKCHANNEL_CTL      = 40
OP_BIND_CONN_TO_SESSION = 41
OP_EXCHANGE_ID          = 42
OP_CREATE_SESSION       = 43
OP_DESTROY_SESSION      = 44
OP_FREE_STATEID         = 45
OP_GET_DIR_DELEGATION   = 46
OP_GETDEVICEINFO        = 47
OP_GETDEVICELIST        = 48     # Mandatory not-to-implement in NFSv4.2
OP_LAYOUTCOMMIT         = 49
OP_LAYOUTGET            = 50
OP_LAYOUTRETURN         = 51
OP_SECINFO_NO_NAME      = 52
OP_SEQUENCE             = 53
OP_SET_SSV              = 54
OP_TEST_STATEID         = 55
OP_WANT_DELEGATION      = 56
OP_DESTROY_CLIENTID     = 57
OP_RECLAIM_COMPLETE     = 58
# New operations for NFSv4.2
OP_ALLOCATE             = 59
OP_COPY                 = 60
OP_COPY_NOTIFY          = 61
OP_DEALLOCATE           = 62
OP_IO_ADVISE            = 63
OP_LAYOUTERROR          = 64
OP_LAYOUTSTATS          = 65
OP_OFFLOAD_CANCEL       = 66
OP_OFFLOAD_STATUS       = 67
OP_READ_PLUS            = 68
OP_SEEK                 = 69
OP_WRITE_SAME           = 70
OP_CLONE                = 71
# Illegal operation
OP_ILLEGAL              = 10044

nfs_opnum4 = {
        3: "OP_ACCESS",
        4: "OP_CLOSE",
        5: "OP_COMMIT",
        6: "OP_CREATE",
        7: "OP_DELEGPURGE",
        8: "OP_DELEGRETURN",
        9: "OP_GETATTR",
       10: "OP_GETFH",
       11: "OP_LINK",
       12: "OP_LOCK",
       13: "OP_LOCKT",
       14: "OP_LOCKU",
       15: "OP_LOOKUP",
       16: "OP_LOOKUPP",
       17: "OP_NVERIFY",
       18: "OP_OPEN",
       19: "OP_OPENATTR",
       20: "OP_OPEN_CONFIRM",
       21: "OP_OPEN_DOWNGRADE",
       22: "OP_PUTFH",
       23: "OP_PUTPUBFH",
       24: "OP_PUTROOTFH",
       25: "OP_READ",
       26: "OP_READDIR",
       27: "OP_READLINK",
       28: "OP_REMOVE",
       29: "OP_RENAME",
       30: "OP_RENEW",
       31: "OP_RESTOREFH",
       32: "OP_SAVEFH",
       33: "OP_SECINFO",
       34: "OP_SETATTR",
       35: "OP_SETCLIENTID",
       36: "OP_SETCLIENTID_CONFIRM",
       37: "OP_VERIFY",
       38: "OP_WRITE",
       39: "OP_RELEASE_LOCKOWNER",
       40: "OP_BACKCHANNEL_CTL",
       41: "OP_BIND_CONN_TO_SESSION",
       42: "OP_EXCHANGE_ID",
       43: "OP_CREATE_SESSION",
       44: "OP_DESTROY_SESSION",
       45: "OP_FREE_STATEID",
       46: "OP_GET_DIR_DELEGATION",
       47: "OP_GETDEVICEINFO",
       48: "OP_GETDEVICELIST",
       49: "OP_LAYOUTCOMMIT",
       50: "OP_LAYOUTGET",
       51: "OP_LAYOUTRETURN",
       52: "OP_SECINFO_NO_NAME",
       53: "OP_SEQUENCE",
       54: "OP_SET_SSV",
       55: "OP_TEST_STATEID",
       56: "OP_WANT_DELEGATION",
       57: "OP_DESTROY_CLIENTID",
       58: "OP_RECLAIM_COMPLETE",
       59: "OP_ALLOCATE",
       60: "OP_COPY",
       61: "OP_COPY_NOTIFY",
       62: "OP_DEALLOCATE",
       63: "OP_IO_ADVISE",
       64: "OP_LAYOUTERROR",
       65: "OP_LAYOUTSTATS",
       66: "OP_OFFLOAD_CANCEL",
       67: "OP_OFFLOAD_STATUS",
       68: "OP_READ_PLUS",
       69: "OP_SEEK",
       70: "OP_WRITE_SAME",
       71: "OP_CLONE",
    10044: "OP_ILLEGAL",
}

ACCESS4_READ    = 0x00000001
ACCESS4_LOOKUP  = 0x00000002
ACCESS4_MODIFY  = 0x00000004
ACCESS4_EXTEND  = 0x00000008
ACCESS4_DELETE  = 0x00000010
ACCESS4_EXECUTE = 0x00000020

# Enum nfs_lock_type4
READ_LT   = 1
WRITE_LT  = 2
READW_LT  = 3  # blocking read
WRITEW_LT = 4  # blocking write

nfs_lock_type4 = {
    1: "READ_LT",
    2: "WRITE_LT",
    3: "READW_LT",
    4: "WRITEW_LT",
}

# Enum createmode4
UNCHECKED4   = 0
GUARDED4     = 1
# Deprecated in NFSv4.1.
EXCLUSIVE4   = 2

# New to NFSv4.1. If session is persistent,
# GUARDED4 MUST be used. Otherwise, use
# EXCLUSIVE4_1 instead of EXCLUSIVE4.
EXCLUSIVE4_1 = 3

createmode4 = {
    0: "UNCHECKED4",
    1: "GUARDED4",
    2: "EXCLUSIVE4",
    3: "EXCLUSIVE4_1",
}

# Enum opentype4
OPEN4_NOCREATE = 0
OPEN4_CREATE   = 1

opentype4 = {
    0: "OPEN4_NOCREATE",
    1: "OPEN4_CREATE",
}

# Enum limit_by4
NFS_LIMIT_SIZE   = 1
NFS_LIMIT_BLOCKS = 2

limit_by4 = {
    1: "NFS_LIMIT_SIZE",
    2: "NFS_LIMIT_BLOCKS",
}

# Share Access and Deny constants for open argument
OPEN4_SHARE_ACCESS_READ                               = 0x00000001
OPEN4_SHARE_ACCESS_WRITE                              = 0x00000002
OPEN4_SHARE_ACCESS_BOTH                               = 0x00000003
OPEN4_SHARE_DENY_NONE                                 = 0x00000000
OPEN4_SHARE_DENY_READ                                 = 0x00000001
OPEN4_SHARE_DENY_WRITE                                = 0x00000002
OPEN4_SHARE_DENY_BOTH                                 = 0x00000003
# New flags for share_access field of OPEN4args
OPEN4_SHARE_ACCESS_WANT_DELEG_MASK                    = 0xFF00
OPEN4_SHARE_ACCESS_WANT_NO_PREFERENCE                 = 0x0000
OPEN4_SHARE_ACCESS_WANT_READ_DELEG                    = 0x0100
OPEN4_SHARE_ACCESS_WANT_WRITE_DELEG                   = 0x0200
OPEN4_SHARE_ACCESS_WANT_ANY_DELEG                     = 0x0300
OPEN4_SHARE_ACCESS_WANT_NO_DELEG                      = 0x0400
OPEN4_SHARE_ACCESS_WANT_CANCEL                        = 0x0500
OPEN4_SHARE_ACCESS_WANT_SIGNAL_DELEG_WHEN_RESRC_AVAIL = 0x10000
OPEN4_SHARE_ACCESS_WANT_PUSH_DELEG_WHEN_UNCONTENDED   = 0x20000

# Enum open_delegation_type4
OPEN_DELEGATE_NONE     = 0
OPEN_DELEGATE_READ     = 1
OPEN_DELEGATE_WRITE    = 2
OPEN_DELEGATE_NONE_EXT = 3  # New to NFSv4.1

open_delegation_type4 = {
    0: "OPEN_DELEGATE_NONE",
    1: "OPEN_DELEGATE_READ",
    2: "OPEN_DELEGATE_WRITE",
    3: "OPEN_DELEGATE_NONE_EXT",
}

# Enum open_claim_type4

# Not a reclaim.
CLAIM_NULL          = 0
CLAIM_PREVIOUS      = 1
CLAIM_DELEGATE_CUR  = 2
CLAIM_DELEGATE_PREV = 3

# Not a reclaim.
# Like CLAIM_NULL, but object identified
# by the current filehandle.
CLAIM_FH            = 4  # New to NFSv4.1

# Like CLAIM_DELEGATE_CUR, but object identified
# by current filehandle.
CLAIM_DELEG_CUR_FH  = 5  # New to NFSv4.1

# Like CLAIM_DELEGATE_PREV, but object identified
# by current filehandle.
CLAIM_DELEG_PREV_FH = 6  # New to NFSv4.1

open_claim_type4 = {
    0: "CLAIM_NULL",
    1: "CLAIM_PREVIOUS",
    2: "CLAIM_DELEGATE_CUR",
    3: "CLAIM_DELEGATE_PREV",
    4: "CLAIM_FH",
    5: "CLAIM_DELEG_CUR_FH",
    6: "CLAIM_DELEG_PREV_FH",
}

# Enum why_no_delegation4
WND4_NOT_WANTED                 = 0
WND4_CONTENTION                 = 1
WND4_RESOURCE                   = 2
WND4_NOT_SUPP_FTYPE             = 3
WND4_WRITE_DELEG_NOT_SUPP_FTYPE = 4
WND4_NOT_SUPP_UPGRADE           = 5
WND4_NOT_SUPP_DOWNGRADE         = 6
WND4_CANCELLED                  = 7
WND4_IS_DIR                     = 8

why_no_delegation4 = {
    0: "WND4_NOT_WANTED",
    1: "WND4_CONTENTION",
    2: "WND4_RESOURCE",
    3: "WND4_NOT_SUPP_FTYPE",
    4: "WND4_WRITE_DELEG_NOT_SUPP_FTYPE",
    5: "WND4_NOT_SUPP_UPGRADE",
    6: "WND4_NOT_SUPP_DOWNGRADE",
    7: "WND4_CANCELLED",
    8: "WND4_IS_DIR",
}
# Result flags
#
# Client must confirm open
OPEN4_RESULT_CONFIRM           = 0x00000002
# Type of file locking behavior at the server
OPEN4_RESULT_LOCKTYPE_POSIX    = 0x00000004
# Server will preserve file if removed while open
OPEN4_RESULT_PRESERVE_UNLINKED = 0x00000008
# Server may use CB_NOTIFY_LOCK on locks derived from this open
OPEN4_RESULT_MAY_NOTIFY_LOCK   = 0x00000020

# Enum nfs_secflavor4
AUTH_NONE  = 0
AUTH_SYS   = 1
RPCSEC_GSS = 6

nfs_secflavor4 = {
    0: "AUTH_NONE",
    1: "AUTH_SYS",
    6: "RPCSEC_GSS",
}

# Enum rpc_gss_svc_t
RPC_GSS_SVC_NONE      = 1
RPC_GSS_SVC_INTEGRITY = 2
RPC_GSS_SVC_PRIVACY   = 3

rpc_gss_svc_t = {
    1: "RPC_GSS_SVC_NONE",
    2: "RPC_GSS_SVC_INTEGRITY",
    3: "RPC_GSS_SVC_PRIVACY",
}

# Enum channel_dir_from_client4
CDFC4_FORE         = 0x1
CDFC4_BACK         = 0x2
CDFC4_FORE_OR_BOTH = 0x3
CDFC4_BACK_OR_BOTH = 0x7

channel_dir_from_client4 = {
    0x1: "CDFC4_FORE",
    0x2: "CDFC4_BACK",
    0x3: "CDFC4_FORE_OR_BOTH",
    0x7: "CDFC4_BACK_OR_BOTH",
}

# Enum channel_dir_from_server4
CDFS4_FORE = 0x1
CDFS4_BACK = 0x2
CDFS4_BOTH = 0x3

channel_dir_from_server4 = {
    0x1: "CDFS4_FORE",
    0x2: "CDFS4_BACK",
    0x3: "CDFS4_BOTH",
}

# EXCHANGE_ID: Instantiate Client ID
# ======================================================================
EXCHGID4_FLAG_SUPP_MOVED_REFER    = 0x00000001
EXCHGID4_FLAG_SUPP_MOVED_MIGR     = 0x00000002
EXCHGID4_FLAG_SUPP_FENCE_OPS      = 0x00000004  # New to NFSv4.2
EXCHGID4_FLAG_BIND_PRINC_STATEID  = 0x00000100
EXCHGID4_FLAG_USE_NON_PNFS        = 0x00010000
EXCHGID4_FLAG_USE_PNFS_MDS        = 0x00020000
EXCHGID4_FLAG_USE_PNFS_DS         = 0x00040000
EXCHGID4_FLAG_MASK_PNFS           = 0x00070000
EXCHGID4_FLAG_UPD_CONFIRMED_REC_A = 0x40000000
EXCHGID4_FLAG_CONFIRMED_R         = 0x80000000

# Enum state_protect_how4
SP4_NONE      = 0
SP4_MACH_CRED = 1
SP4_SSV       = 2

state_protect_how4 = {
    0: "SP4_NONE",
    1: "SP4_MACH_CRED",
    2: "SP4_SSV",
}

CREATE_SESSION4_FLAG_PERSIST        = 0x00000001
CREATE_SESSION4_FLAG_CONN_BACK_CHAN = 0x00000002
CREATE_SESSION4_FLAG_CONN_RDMA      = 0x00000004

# Enum gddrnf4_status
GDD4_OK      = 0
GDD4_UNAVAIL = 1

gddrnf4_status = {
    0: "GDD4_OK",
    1: "GDD4_UNAVAIL",
}

# Enum secinfo_style4
SECINFO_STYLE4_CURRENT_FH = 0
SECINFO_STYLE4_PARENT     = 1

secinfo_style4 = {
    0: "SECINFO_STYLE4_CURRENT_FH",
    1: "SECINFO_STYLE4_PARENT",
}

SEQ4_STATUS_CB_PATH_DOWN               = 0x00000001
SEQ4_STATUS_CB_GSS_CONTEXTS_EXPIRING   = 0x00000002
SEQ4_STATUS_CB_GSS_CONTEXTS_EXPIRED    = 0x00000004
SEQ4_STATUS_EXPIRED_ALL_STATE_REVOKED  = 0x00000008
SEQ4_STATUS_EXPIRED_SOME_STATE_REVOKED = 0x00000010
SEQ4_STATUS_ADMIN_STATE_REVOKED        = 0x00000020
SEQ4_STATUS_RECALLABLE_STATE_REVOKED   = 0x00000040
SEQ4_STATUS_LEASE_MOVED                = 0x00000080
SEQ4_STATUS_RESTART_RECLAIM_NEEDED     = 0x00000100
SEQ4_STATUS_CB_PATH_DOWN_SESSION       = 0x00000200
SEQ4_STATUS_BACKCHANNEL_FAULT          = 0x00000400
SEQ4_STATUS_DEVID_CHANGED              = 0x00000800
SEQ4_STATUS_DEVID_DELETED              = 0x00001000

# Enum IO_ADVISE_type4
IO_ADVISE4_NORMAL                 = 0
IO_ADVISE4_SEQUENTIAL             = 1
IO_ADVISE4_SEQUENTIAL_BACKWARDS   = 2
IO_ADVISE4_RANDOM                 = 3
IO_ADVISE4_WILLNEED               = 4
IO_ADVISE4_WILLNEED_OPPORTUNISTIC = 5
IO_ADVISE4_DONTNEED               = 6
IO_ADVISE4_NOREUSE                = 7
IO_ADVISE4_READ                   = 8
IO_ADVISE4_WRITE                  = 9
IO_ADVISE4_INIT_PROXIMITY         = 10

IO_ADVISE_type4 = {
     0: "IO_ADVISE4_NORMAL",
     1: "IO_ADVISE4_SEQUENTIAL",
     2: "IO_ADVISE4_SEQUENTIAL_BACKWARDS",
     3: "IO_ADVISE4_RANDOM",
     4: "IO_ADVISE4_WILLNEED",
     5: "IO_ADVISE4_WILLNEED_OPPORTUNISTIC",
     6: "IO_ADVISE4_DONTNEED",
     7: "IO_ADVISE4_NOREUSE",
     8: "IO_ADVISE4_READ",
     9: "IO_ADVISE4_WRITE",
    10: "IO_ADVISE4_INIT_PROXIMITY",
}

# Enum data_content4
NFS4_CONTENT_DATA = 0
NFS4_CONTENT_HOLE = 1

data_content4 = {
    0: "NFS4_CONTENT_DATA",
    1: "NFS4_CONTENT_HOLE",
}

# Enum nfs_cb_opnum4
OP_CB_GETATTR              = 3
OP_CB_RECALL               = 4
# Callback operations new to NFSv4.1
OP_CB_LAYOUTRECALL         = 5
OP_CB_NOTIFY               = 6
OP_CB_PUSH_DELEG           = 7
OP_CB_RECALL_ANY           = 8
OP_CB_RECALLABLE_OBJ_AVAIL = 9
OP_CB_RECALL_SLOT          = 10
OP_CB_SEQUENCE             = 11
OP_CB_WANTS_CANCELLED      = 12
OP_CB_NOTIFY_LOCK          = 13
OP_CB_NOTIFY_DEVICEID      = 14
# Callback operations new to NFSv4.2
OP_CB_OFFLOAD              = 15
# Illegal callback operation
OP_CB_ILLEGAL              = 10044

nfs_cb_opnum4 = {
        3: "OP_CB_GETATTR",
        4: "OP_CB_RECALL",
        5: "OP_CB_LAYOUTRECALL",
        6: "OP_CB_NOTIFY",
        7: "OP_CB_PUSH_DELEG",
        8: "OP_CB_RECALL_ANY",
        9: "OP_CB_RECALLABLE_OBJ_AVAIL",
       10: "OP_CB_RECALL_SLOT",
       11: "OP_CB_SEQUENCE",
       12: "OP_CB_WANTS_CANCELLED",
       13: "OP_CB_NOTIFY_LOCK",
       14: "OP_CB_NOTIFY_DEVICEID",
       15: "OP_CB_OFFLOAD",
    10044: "OP_CB_ILLEGAL",
}

# Enum layoutrecall_type4
LAYOUTRECALL4_FILE = LAYOUT4_RET_REC_FILE
LAYOUTRECALL4_FSID = LAYOUT4_RET_REC_FSID
LAYOUTRECALL4_ALL  = LAYOUT4_RET_REC_ALL

layoutrecall_type4 = {
    LAYOUT4_RET_REC_FILE : "LAYOUTRECALL4_FILE",
    LAYOUT4_RET_REC_FSID : "LAYOUTRECALL4_FSID",
     LAYOUT4_RET_REC_ALL : "LAYOUTRECALL4_ALL",
}

# Enum notify_type4
NOTIFY4_CHANGE_CHILD_ATTRS     = 0
NOTIFY4_CHANGE_DIR_ATTRS       = 1
NOTIFY4_REMOVE_ENTRY           = 2
NOTIFY4_ADD_ENTRY              = 3
NOTIFY4_RENAME_ENTRY           = 4
NOTIFY4_CHANGE_COOKIE_VERIFIER = 5

notify_type4 = {
    0: "NOTIFY4_CHANGE_CHILD_ATTRS",
    1: "NOTIFY4_CHANGE_DIR_ATTRS",
    2: "NOTIFY4_REMOVE_ENTRY",
    3: "NOTIFY4_ADD_ENTRY",
    4: "NOTIFY4_RENAME_ENTRY",
    5: "NOTIFY4_CHANGE_COOKIE_VERIFIER",
}

# CB_RECALL_ANY: Keep Any N Recallable Objects
# ======================================================================
RCA4_TYPE_MASK_RDATA_DLG        = 0
RCA4_TYPE_MASK_WDATA_DLG        = 1
RCA4_TYPE_MASK_DIR_DLG          = 2
RCA4_TYPE_MASK_FILE_LAYOUT      = 3
RCA4_TYPE_MASK_BLK_LAYOUT       = 4
RCA4_TYPE_MASK_OBJ_LAYOUT_MIN   = 8
RCA4_TYPE_MASK_OBJ_LAYOUT_MAX   = 9
RCA4_TYPE_MASK_OTHER_LAYOUT_MIN = 12
RCA4_TYPE_MASK_OTHER_LAYOUT_MAX = 15

# Enum notify_deviceid_type4
NOTIFY_DEVICEID4_CHANGE = 1
NOTIFY_DEVICEID4_DELETE = 2

notify_deviceid_type4 = {
    1: "NOTIFY_DEVICEID4_CHANGE",
    2: "NOTIFY_DEVICEID4_DELETE",
}
