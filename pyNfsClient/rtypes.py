from . import const


class specdata3:
    # XDR definition:
    # struct specdata3 {
    #     uint32 major;
    #     uint32 minor;
    # };
    def __init__(self, major=None, minor=None):
        self.major = major
        self.minor = minor

    def __repr__(self):
        out = []
        if self.major is not None:
            out += ['major=%s' % repr(self.major)]
        if self.minor is not None:
            out += ['minor=%s' % repr(self.minor)]
        return 'specdata3(%s)' % ', '.join(out)


class nfs_fh3:
    # XDR definition:
    # struct nfs_fh3 {
    #     opaque data<NFS3_FHSIZE>;
    # };
    def __init__(self, data=None):
        self.data = data

    def __repr__(self):
        out = []
        if self.data is not None:
            out += ['data=%s' % repr(self.data)]
        return 'nfs_fh3(%s)' % ', '.join(out)


class nfstime3:
    # XDR definition:
    # struct nfstime3 {
    #     uint32 seconds;
    #     uint32 nseconds;
    # };
    def __init__(self, seconds=None, nseconds=None):
        self.seconds = seconds
        self.nseconds = nseconds

    def __repr__(self):
        out = []
        if self.seconds is not None:
            out += ['seconds=%s' % repr(self.seconds)]
        if self.nseconds is not None:
            out += ['nseconds=%s' % repr(self.nseconds)]
        return 'nfstime3(%s)' % ', '.join(out)


class fattr3:
    # XDR definition:
    # struct fattr3 {
    #     ftype3 type;
    #     uint32 mode;
    #     uint32 nlink;
    #     uint32 uid;
    #     uint32 gid;
    #     uint64 size;
    #     uint64 used;
    #     specdata3 rdev;
    #     uint64 fsid;
    #     uint64 fileid;
    #     nfstime3 atime;
    #     nfstime3 mtime;
    #     nfstime3 ctime;
    # };
    def __init__(self, type=None, mode=None, nlink=None, uid=None, gid=None, size=None, used=None, rdev=None,
                 fsid=None, fileid=None, atime=None, mtime=None, ctime=None):
        self.type = type
        self.mode = mode
        self.nlink = nlink
        self.uid = uid
        self.gid = gid
        self.size = size
        self.used = used
        self.rdev = rdev
        self.fsid = fsid
        self.fileid = fileid
        self.atime = atime
        self.mtime = mtime
        self.ctime = ctime

    def __repr__(self):
        out = []
        if self.type is not None:
            out += ['type=%s' % const.FTYPE3.get(self.type, self.type)]
        if self.mode is not None:
            out += ['mode=%s' % repr(self.mode)]
        if self.nlink is not None:
            out += ['nlink=%s' % repr(self.nlink)]
        if self.uid is not None:
            out += ['uid=%s' % repr(self.uid)]
        if self.gid is not None:
            out += ['gid=%s' % repr(self.gid)]
        if self.size is not None:
            out += ['size=%s' % repr(self.size)]
        if self.used is not None:
            out += ['used=%s' % repr(self.used)]
        if self.rdev is not None:
            out += ['rdev=%s' % repr(self.rdev)]
        if self.fsid is not None:
            out += ['fsid=%s' % repr(self.fsid)]
        if self.fileid is not None:
            out += ['fileid=%s' % repr(self.fileid)]
        if self.atime is not None:
            out += ['atime=%s' % repr(self.atime)]
        if self.mtime is not None:
            out += ['mtime=%s' % repr(self.mtime)]
        if self.ctime is not None:
            out += ['ctime=%s' % repr(self.ctime)]
        return 'fattr3(%s)' % ', '.join(out)


class post_op_attr:
    # XDR definition:
    # union post_op_attr switch(bool present) {
    #     case TRUE:
    #         fattr3 attributes;
    #     case FALSE:
    #         void;
    # };
    def __init__(self, present=None, attributes=None):
        self.present = present
        self.attributes = attributes

    def __repr__(self):
        out = []
        if self.present is not None:
            out += ['present=%s' % repr(self.present)]
        if self.attributes is not None:
            out += ['attributes=%s' % repr(self.attributes)]
        return 'post_op_attr(%s)' % ', '.join(out)


class wcc_attr:
    # XDR definition:
    # struct wcc_attr {
    #     uint64 size;
    #     nfstime3 mtime;
    #     nfstime3 ctime;
    # };
    def __init__(self, size=None, mtime=None, ctime=None):
        self.size = size
        self.mtime = mtime
        self.ctime = ctime

    def __repr__(self):
        out = []
        if self.size is not None:
            out += ['size=%s' % repr(self.size)]
        if self.mtime is not None:
            out += ['mtime=%s' % repr(self.mtime)]
        if self.ctime is not None:
            out += ['ctime=%s' % repr(self.ctime)]
        return 'wcc_attr(%s)' % ', '.join(out)


class pre_op_attr:
    # XDR definition:
    # union pre_op_attr switch(bool present) {
    #     case TRUE:
    #         wcc_attr attributes;
    #     case FALSE:
    #         void;
    # };
    def __init__(self, present=None, attributes=None):
        self.present = present
        self.attributes = attributes

    def __repr__(self):
        out = []
        if self.present is not None:
            out += ['present=%s' % repr(self.present)]
        if self.attributes is not None:
            out += ['attributes=%s' % repr(self.attributes)]
        return 'pre_op_attr(%s)' % ', '.join(out)


class wcc_data:
    # XDR definition:
    # struct wcc_data {
    #     pre_op_attr before;
    #     post_op_attr after;
    # };
    def __init__(self, before=None, after=None):
        self.before = before
        self.after = after

    def __repr__(self):
        out = []
        if self.before is not None:
            out += ['before=%s' % repr(self.before)]
        if self.after is not None:
            out += ['after=%s' % repr(self.after)]
        return 'wcc_data(%s)' % ', '.join(out)


class post_op_fh3:
    # XDR definition:
    # union post_op_fh3 switch(bool present) {
    #     case TRUE:
    #         nfs_fh3 handle;
    #     case FALSE:
    #         void;
    # };
    def __init__(self, present=None, handle=None):
        self.present = present
        self.handle = handle

    def __repr__(self):
        out = []
        if self.present is not None:
            out += ['present=%s' % repr(self.present)]
        if self.handle is not None:
            out += ['handle=%s' % repr(self.handle)]
        return 'post_op_fh3(%s)' % ', '.join(out)


class set_uint32:
    # XDR definition:
    # union set_uint32 switch(bool set) {
    #     case TRUE:
    #         uint32 val;
    #     default:
    #         void;
    # };
    def __init__(self, set=None, val=None):
        self.set = set
        self.val = val

    def __repr__(self):
        out = []
        if self.set is not None:
            out += ['set=%s' % repr(self.set)]
        if self.val is not None:
            out += ['val=%s' % repr(self.val)]
        return 'set_uint32(%s)' % ', '.join(out)


class set_uint64:
    # XDR definition:
    # union set_uint64 switch(bool set) {
    #     case TRUE:
    #         uint64 val;
    #     default:
    #         void;
    # };
    def __init__(self, set=None, val=None):
        self.set = set
        self.val = val

    def __repr__(self):
        out = []
        if self.set is not None:
            out += ['set=%s' % repr(self.set)]
        if self.val is not None:
            out += ['val=%s' % repr(self.val)]
        return 'set_uint64(%s)' % ', '.join(out)


class set_time:
    # XDR definition:
    # union set_time switch(time_how set) {
    #     case SET_TO_CLIENT_TIME:
    #         nfstime3 time;
    #     default:
    #         void;
    # };
    def __init__(self, set=None, time=None):
        self.set = set
        self.time = time

    def __repr__(self):
        out = []
        if self.set is not None:
            out += ['set=%s' % const.time_how.get(self.set, self.set)]
        if self.time is not None:
            out += ['time=%s' % repr(self.time)]
        return 'set_time(%s)' % ', '.join(out)


class sattr3:
    # XDR definition:
    # struct sattr3 {
    #     set_uint32 mode;
    #     set_uint32 uid;
    #     set_uint32 gid;
    #     set_uint64 size;
    #     set_time atime;
    #     set_time mtime;
    # };
    def __init__(self, mode=None, uid=None, gid=None, size=None, atime=None, mtime=None):
        self.mode = mode
        self.uid = uid
        self.gid = gid
        self.size = size
        self.atime = atime
        self.mtime = mtime

    def __repr__(self):
        out = []
        if self.mode is not None:
            out += ['mode=%s' % repr(self.mode)]
        if self.uid is not None:
            out += ['uid=%s' % repr(self.uid)]
        if self.gid is not None:
            out += ['gid=%s' % repr(self.gid)]
        if self.size is not None:
            out += ['size=%s' % repr(self.size)]
        if self.atime is not None:
            out += ['atime=%s' % repr(self.atime)]
        if self.mtime is not None:
            out += ['mtime=%s' % repr(self.mtime)]
        return 'sattr3(%s)' % ', '.join(out)


class diropargs3:
    # XDR definition:
    # struct diropargs3 {
    #     nfs_fh3 dir;
    #     filename3 name;
    # };
    def __init__(self, dir=None, name=None):
        self.dir = dir
        self.name = name

    def __repr__(self):
        out = []
        if self.dir is not None:
            out += ['dir=%s' % repr(self.dir)]
        if self.name is not None:
            out += ['name=%s' % repr(self.name)]
        return 'diropargs3(%s)' % ', '.join(out)


class diropres3ok:
    # XDR definition:
    # struct diropres3ok {
    #     post_op_fh3 obj;
    #     post_op_attr obj_attributes;
    #     wcc_data dir_wcc;
    # };
    def __init__(self, obj=None, obj_attributes=None, dir_wcc=None):
        self.obj = obj
        self.obj_attributes = obj_attributes
        self.dir_wcc = dir_wcc

    def __repr__(self):
        out = []
        if self.obj is not None:
            out += ['obj=%s' % repr(self.obj)]
        if self.obj_attributes is not None:
            out += ['obj_attributes=%s' % repr(self.obj_attributes)]
        if self.dir_wcc is not None:
            out += ['dir_wcc=%s' % repr(self.dir_wcc)]
        return 'diropres3ok(%s)' % ', '.join(out)


class diropres3:
    # XDR definition:
    # union diropres3 switch(nfsstat3 status) {
    #     case NFS3_OK:
    #         diropres3ok resok;
    #     default:
    #         wcc_data resfail;
    # };
    def __init__(self, status=None, resok=None):
        self.status = status
        self.resok = resok

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.resok is not None:
            out += ['resok=%s' % repr(self.resok)]
        return 'diropres3(%s)' % ', '.join(out)


class wccstat3:
    # XDR definition:
    # union wccstat3 switch(nfsstat3 status) {
    #     case -1:
    #         void;
    #     default:
    #         wcc_data wcc;
    # };
    def __init__(self, status=None):
        self.status = status

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        return 'wccstat3(%s)' % ', '.join(out)


class getattr3res:
    # XDR definition:
    # union getattr3res switch(nfsstat3 status) {
    #     case NFS3_OK:
    #         fattr3 attributes;
    #     default:
    #         void;
    # };
    def __init__(self, status=None, attributes=None):
        self.status = status
        self.attributes = attributes

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.attributes is not None:
            out += ['attributes=%s' % repr(self.attributes)]
        return 'getattr3res(%s)' % ', '.join(out)


class sattrguard3:
    # XDR definition:
    # union sattrguard3 switch(bool check) {
    #     case TRUE:
    #         nfstime3 ctime;
    #     case FALSE:
    #         void;
    # };
    def __init__(self, check=None, ctime=None):
        self.check = check
        self.ctime = ctime

    def __repr__(self):
        out = []
        if self.check is not None:
            out += ['check=%s' % repr(self.check)]
        if self.ctime is not None:
            out += ['ctime=%s' % repr(self.ctime)]
        return 'sattrguard3(%s)' % ', '.join(out)


class setattr3args:
    # XDR definition:
    # struct setattr3args {
    #     nfs_fh3 object;
    #     sattr3 new_attributes;
    #     sattrguard3 guard;
    # };
    def __init__(self, object=None, new_attributes=None, guard=None):
        self.object = object
        self.new_attributes = new_attributes
        self.guard = guard

    def __repr__(self):
        out = []
        if self.object is not None:
            out += ['object=%s' % repr(self.object)]
        if self.new_attributes is not None:
            out += ['new_attributes=%s' % repr(self.new_attributes)]
        if self.guard is not None:
            out += ['guard=%s' % repr(self.guard)]
        return 'setattr3args(%s)' % ', '.join(out)


class wcc_data3res:
    def __init__(self, category, status=None, wcc_data=None):
        self.category = str(category) or ''
        self.status = status
        self.wcc_data = wcc_data

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.wcc_data is not None:
            out += ['wcc_data=%s' % repr(self.wcc_data)]
        return self.category + '(%s)' % ', '.join(out)


class lookup3resok:
    # XDR definition:
    # struct lookup3resok {
    #     nfs_fh3 object;
    #     post_op_attr obj_attributes;
    #     post_op_attr dir_attributes;
    # };
    def __init__(self, object=None, obj_attributes=None, dir_attributes=None):
        self.object = object
        self.obj_attributes = obj_attributes
        self.dir_attributes = dir_attributes

    def __repr__(self):
        out = []
        if self.object is not None:
            out += ['object=%s' % repr(self.object)]
        if self.obj_attributes is not None:
            out += ['obj_attributes=%s' % repr(self.obj_attributes)]
        if self.dir_attributes is not None:
            out += ['dir_attributes=%s' % repr(self.dir_attributes)]
        return 'lookup3resok(%s)' % ', '.join(out)


class lookup3res:
    # XDR definition:
    # union lookup3res switch(nfsstat3 status) {
    #     case NFS3_OK:
    #         lookup3resok resok;
    #     default:
    #         post_op_attr resfail;
    # };
    def __init__(self, status=None, resok=None):
        self.status = status
        self.resok = resok

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.resok is not None:
            out += ['resok=%s' % repr(self.resok)]
        return 'lookup3res(%s)' % ', '.join(out)


class access3args:
    # XDR definition:
    # struct access3args {
    #     nfs_fh3 object;
    #     uint32 access;
    # };
    def __init__(self, object=None, access=None):
        self.object = object
        self.access = access

    def __repr__(self):
        out = []
        if self.object is not None:
            out += ['object=%s' % repr(self.object)]
        if self.access is not None:
            out += ['access=%s' % repr(self.access)]
        return 'access3args(%s)' % ', '.join(out)


class access3resok:
    # XDR definition:
    # struct access3resok {
    #     post_op_attr obj_attributes;
    #     uint32 access;
    # };
    def __init__(self, obj_attributes=None, access=None):
        self.obj_attributes = obj_attributes
        self.access = access

    def __repr__(self):
        out = []
        if self.obj_attributes is not None:
            out += ['obj_attributes=%s' % repr(self.obj_attributes)]
        if self.access is not None:
            out += ['access=%s' % repr(self.access)]
        return 'access3resok(%s)' % ', '.join(out)


class access3res:
    # XDR definition:
    # union access3res switch(nfsstat3 status) {
    #     case NFS3_OK:
    #         access3resok resok;
    #     default:
    #         post_op_attr resfail;
    # };
    def __init__(self, status=None, resok=None):
        self.status = status
        self.resok = resok

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.resok is not None:
            out += ['resok=%s' % repr(self.resok)]
        return 'access3res(%s)' % ', '.join(out)


class readlink3resok:
    # XDR definition:
    # struct readlink3resok {
    #     post_op_attr symlink_attributes;
    #     nfspath3 data;
    # };
    def __init__(self, symlink_attributes=None, data=None):
        self.symlink_attributes = symlink_attributes
        self.data = data

    def __repr__(self):
        out = []
        if self.symlink_attributes is not None:
            out += ['symlink_attributes=%s' % repr(self.symlink_attributes)]
        if self.data is not None:
            out += ['data=%s' % repr(self.data)]
        return 'readlink3resok(%s)' % ', '.join(out)


class readlink3res:
    # XDR definition:
    # union readlink3res switch(nfsstat3 status) {
    #     case NFS3_OK:
    #         readlink3resok resok;
    #     default:
    #         post_op_attr resfail;
    # };
    def __init__(self, status=None, resok=None):
        self.status = status
        self.resok = resok

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.resok is not None:
            out += ['resok=%s' % repr(self.resok)]
        return 'readlink3res(%s)' % ', '.join(out)


class read3args:
    # XDR definition:
    # struct read3args {
    #     nfs_fh3 file;
    #     uint64 offset;
    #     uint32 count;
    # };
    def __init__(self, file=None, offset=None, count=None):
        self.file = file
        self.offset = offset
        self.count = count

    def __repr__(self):
        out = []
        if self.file is not None:
            out += ['file=%s' % repr(self.file)]
        if self.offset is not None:
            out += ['offset=%s' % repr(self.offset)]
        if self.count is not None:
            out += ['count=%s' % repr(self.count)]
        return 'read3args(%s)' % ', '.join(out)


class read3resok:
    # XDR definition:
    # struct read3resok {
    #     post_op_attr file_attributes;
    #     uint32 count;
    #     bool eof;
    #     opaque data<>;
    # };
    def __init__(self, file_attributes=None, count=None, eof=None, data=None):
        self.file_attributes = file_attributes
        self.count = count
        self.eof = eof
        self.data = data

    def __repr__(self):
        out = []
        if self.file_attributes is not None:
            out += ['file_attributes=%s' % repr(self.file_attributes)]
        if self.count is not None:
            out += ['count=%s' % repr(self.count)]
        if self.eof is not None:
            out += ['eof=%s' % repr(self.eof)]
        if self.data is not None:
            out += ['data=%s' % repr(self.data)]
        return 'read3resok(%s)' % ', '.join(out)


class read3res:
    # XDR definition:
    # union read3res switch(nfsstat3 status) {
    #     case NFS3_OK:
    #         read3resok resok;
    #     default:
    #         post_op_attr resfail;
    # };
    def __init__(self, status=None, resok=None):
        self.status = status
        self.resok = resok

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.resok is not None:
            out += ['resok=%s' % repr(self.resok)]
        return 'read3res(%s)' % ', '.join(out)


class write3args:
    # XDR definition:
    # struct write3args {
    #     nfs_fh3 file;
    #     uint64 offset;
    #     uint32 count;
    #     stable_how stable;
    #     opaque data<>;
    # };
    def __init__(self, file=None, offset=None, count=None, stable=None, data=None):
        self.file = file
        self.offset = offset
        self.count = count
        self.stable = stable
        self.data = data

    def __repr__(self):
        out = []
        if self.file is not None:
            out += ['file=%s' % repr(self.file)]
        if self.offset is not None:
            out += ['offset=%s' % repr(self.offset)]
        if self.count is not None:
            out += ['count=%s' % repr(self.count)]
        if self.stable is not None:
            out += ['stable=%s' % const.STABLE_HOW.get(self.stable, self.stable)]
        if self.data is not None:
            out += ['data=%s' % repr(self.data)]
        return 'write3args(%s)' % ', '.join(out)


class write3resok:
    # XDR definition:
    # struct write3resok {
    #     wcc_data file_wcc;
    #     uint32 count;
    #     stable_how committed;
    #     writeverf3 verf;
    # };
    def __init__(self, file_wcc=None, count=None, committed=None, verf=None):
        self.file_wcc = file_wcc
        self.count = count
        self.committed = committed
        self.verf = verf

    def __repr__(self):
        out = []
        if self.file_wcc is not None:
            out += ['file_wcc=%s' % repr(self.file_wcc)]
        if self.count is not None:
            out += ['count=%s' % repr(self.count)]
        if self.committed is not None:
            out += ['committed=%s' % const.STABLE_HOW.get(self.committed, self.committed)]
        if self.verf is not None:
            out += ['verf=%s' % repr(self.verf)]
        return 'write3resok(%s)' % ', '.join(out)


class write3res:
    # XDR definition:
    # union write3res switch(nfsstat3 status) {
    #     case NFS3_OK:
    #         write3resok resok;
    #     default:
    #         wcc_data resfail;
    # };
    def __init__(self, status=None, resok=None):
        self.status = status
        self.resok = resok

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.resok is not None:
            out += ['resok=%s' % repr(self.resok)]
        return 'write3res(%s)' % ', '.join(out)


class createhow3:
    # XDR definition:
    # union createhow3 switch(createmode3 mode) {
    #     case UNCHECKED:
    #     case GUARDED:
    #         sattr3 obj_attributes;
    #     case EXCLUSIVE:
    #         createverf3 verf;
    # };
    def __init__(self, mode=None, obj_attributes=None, verf=None):
        self.mode = mode
        self.obj_attributes = obj_attributes
        self.verf = verf

    def __repr__(self):
        out = []
        if self.mode is not None:
            out += ['mode=%s' % const.CREATEMODE3.get(self.mode, self.mode)]
        if self.obj_attributes is not None:
            out += ['obj_attributes=%s' % repr(self.obj_attributes)]
        if self.verf is not None:
            out += ['verf=%s' % repr(self.verf)]
        return 'createhow3(%s)' % ', '.join(out)


class create3args:
    # XDR definition:
    # struct create3args {
    #     diropargs3 where;
    #     createhow3 how;
    # };
    def __init__(self, where=None, how=None):
        self.where = where
        self.how = how

    def __repr__(self):
        out = []
        if self.where is not None:
            out += ['where=%s' % repr(self.where)]
        if self.how is not None:
            out += ['how=%s' % repr(self.how)]
        return 'create3args(%s)' % ', '.join(out)


class create3resok:
    # XDR definition:
    # struct CREATE3resok {
    #     post_op_fh3 obj;
    #     post_op_attr obj_attributes;
    #     wcc_data dir_wcc;
    # };
    def __init__(self, obj=None, obj_attributes=None, dir_wcc=None):
        self.obj = obj
        self.obj_attributes = obj_attributes
        self.dir_wcc = dir_wcc

    def __repr__(self):
        out = list()
        if self.obj is not None:
            out += ['obj=%s' % repr(self.obj)]
        if self.obj_attributes is not None:
            out += ['obj_attributes=%s' % repr(self.obj_attributes)]
        if self.dir_wcc is not None:
            out += ['dir_wcc=%s' % repr(self.dir_wcc)]
        return 'create3resok(%s)' % ', '.join(out)


class create3res:
    # XDR definition:
    # union CREATE3res switch (nfsstat3 status) {
    #     case NFS3_OK:
    #         CREATE3resok resok;
    #     default:
    #         CREATE3resfail resfail;
    # };
    def __init__(self, status=None, resok=None):
        self.status = status
        self.resok = resok

    def __repr__(self):
        out = list()
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.resok is not None:
            out += ['resok=%s' % repr(self.resok)]
        return 'create3res(%s)' % ', '.join(out)


class mkdir3args:
    # XDR definition:
    # struct mkdir3args {
    #     diropargs3 where;
    #     sattr3 attributes;
    # };
    def __init__(self, where=None, attributes=None):
        self.where = where
        self.attributes = attributes

    def __repr__(self):
        out = []
        if self.where is not None:
            out += ['where=%s' % repr(self.where)]
        if self.attributes is not None:
            out += ['attributes=%s' % repr(self.attributes)]
        return 'mkdir3args(%s)' % ', '.join(out)


class symlinkdata3:
    # XDR definition:
    # struct symlinkdata3 {
    #     sattr3 symlink_attributes;
    #     nfspath3 symlink_data;
    # };
    def __init__(self, symlink_attributes=None, symlink_data=None):
        self.symlink_attributes = symlink_attributes
        self.symlink_data = symlink_data

    def __repr__(self):
        out = []
        if self.symlink_attributes is not None:
            out += ['symlink_attributes=%s' % repr(self.symlink_attributes)]
        if self.symlink_data is not None:
            out += ['symlink_data=%s' % repr(self.symlink_data)]
        return 'symlinkdata3(%s)' % ', '.join(out)


class symlink3args:
    # XDR definition:
    # struct symlink3args {
    #     diropargs3 where;
    #     symlinkdata3 symlink;
    # };
    def __init__(self, where=None, symlink=None):
        self.where = where
        self.symlink = symlink

    def __repr__(self):
        out = []
        if self.where is not None:
            out += ['where=%s' % repr(self.where)]
        if self.symlink is not None:
            out += ['symlink=%s' % repr(self.symlink)]
        return 'symlink3args(%s)' % ', '.join(out)


class devicedata3:
    # XDR definition:
    # struct devicedata3 {
    #     sattr3 dev_attributes;
    #     specdata3 spec;
    # };
    def __init__(self, dev_attributes=None, spec=None):
        self.dev_attributes = dev_attributes
        self.spec = spec

    def __repr__(self):
        out = []
        if self.dev_attributes is not None:
            out += ['dev_attributes=%s' % repr(self.dev_attributes)]
        if self.spec is not None:
            out += ['spec=%s' % repr(self.spec)]
        return 'devicedata3(%s)' % ', '.join(out)


class mknoddata3:
    # XDR definition:
    # union mknoddata3 switch(ftype3 type) {
    #     case NF3CHR:
    #     case NF3BLK:
    #         devicedata3 device;
    #     case NF3SOCK:
    #     case NF3FIFO:
    #         sattr3 pipe_attributes;
    #     default:
    #         void;
    # };
    def __init__(self, type=None, device=None, pipe_attributes=None):
        self.type = type
        self.device = device
        self.pipe_attributes = pipe_attributes

    def __repr__(self):
        out = []
        if self.type is not None:
            out += ['type=%s' % const.FTYPE3.get(self.type, self.type)]
        if self.device is not None:
            out += ['device=%s' % repr(self.device)]
        if self.pipe_attributes is not None:
            out += ['pipe_attributes=%s' % repr(self.pipe_attributes)]
        return 'mknoddata3(%s)' % ', '.join(out)


class mknod3args:
    # XDR definition:
    # struct mknod3args {
    #     diropargs3 where;
    #     mknoddata3 what;
    # };
    def __init__(self, where=None, what=None):
        self.where = where
        self.what = what

    def __repr__(self):
        out = []
        if self.where is not None:
            out += ['where=%s' % repr(self.where)]
        if self.what is not None:
            out += ['what=%s' % repr(self.what)]
        return 'mknod3args(%s)' % ', '.join(out)


class rename3args:
    # XDR definition:
    # struct rename3args {
    #     diropargs3 from;
    #     diropargs3 to;
    # };
    def __init__(self, from_v=None, to=None):
        self.from_v = from_v
        self.to = to

    def __repr__(self):
        out = []
        if self.from_v is not None:
            out += ['from=%s' % repr(self.from_v)]
        if self.to is not None:
            out += ['to=%s' % repr(self.to)]
        return 'rename3args(%s)' % ', '.join(out)


class rename3wcc:
    # XDR definition:
    # struct rename3wcc {
    #     wcc_data fromdir_wcc;
    #     wcc_data todir_wcc;
    # };
    def __init__(self, fromdir_wcc=None, todir_wcc=None):
        self.fromdir_wcc = fromdir_wcc
        self.todir_wcc = todir_wcc

    def __repr__(self):
        out = []
        if self.fromdir_wcc is not None:
            out += ['fromdir_wcc=%s' % repr(self.fromdir_wcc)]
        if self.todir_wcc is not None:
            out += ['todir_wcc=%s' % repr(self.todir_wcc)]
        return 'rename3wcc(%s)' % ', '.join(out)


class rename3res:
    # XDR definition:
    # union rename3res switch(nfsstat3 status) {
    #     case -1:
    #         void;
    #     default:
    #         rename3wcc res;
    # };
    def __init__(self, status=None):
        self.status = status

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        return 'rename3res(%s)' % ', '.join(out)


class link3args:
    # XDR definition:
    # struct link3args {
    #     nfs_fh3 file;
    #     diropargs3 link;
    # };
    def __init__(self, file=None, link=None):
        self.file = file
        self.link = link

    def __repr__(self):
        out = []
        if self.file is not None:
            out += ['file=%s' % repr(self.file)]
        if self.link is not None:
            out += ['link=%s' % repr(self.link)]
        return 'link3args(%s)' % ', '.join(out)


class link3wcc:
    # XDR definition:
    # struct link3wcc {
    #     post_op_attr file_attributes;
    #     wcc_data linkdir_wcc;
    # };
    def __init__(self, file_attributes=None, linkdir_wcc=None):
        self.file_attributes = file_attributes
        self.linkdir_wcc = linkdir_wcc

    def __repr__(self):
        out = []
        if self.file_attributes is not None:
            out += ['file_attributes=%s' % repr(self.file_attributes)]
        if self.linkdir_wcc is not None:
            out += ['linkdir_wcc=%s' % repr(self.linkdir_wcc)]
        return 'link3wcc(%s)' % ', '.join(out)


class link3res:
    # XDR definition:
    # union link3res switch(nfsstat3 status) {
    #     case -1:
    #         void;
    #     default:
    #         link3wcc res;
    # };
    def __init__(self, status=None):
        self.status = status

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        return 'link3res(%s)' % ', '.join(out)


class readdir3args:
    # XDR definition:
    # struct readdir3args {
    #     nfs_fh3 dir;
    #     uint64 cookie;
    #     cookieverf3 cookieverf;
    #     uint32 count;
    # };
    def __init__(self, dir=None, cookie=None, cookieverf=None, count=None):
        self.dir = dir
        self.cookie = cookie
        self.cookieverf = cookieverf
        self.count = count

    def __repr__(self):
        out = []
        if self.dir is not None:
            out += ['dir=%s' % repr(self.dir)]
        if self.cookie is not None:
            out += ['cookie=%s' % repr(self.cookie)]
        if self.cookieverf is not None:
            out += ['cookieverf=%s' % repr(self.cookieverf)]
        if self.count is not None:
            out += ['count=%s' % repr(self.count)]
        return 'readdir3args(%s)' % ', '.join(out)


class entry3:
    # XDR definition:
    # struct entry3 {
    #     uint64 fileid;
    #     filename3 name;
    #     uint64 cookie;
    #     entry3 nextentry<1>;
    # };
    def __init__(self, fileid=None, name=None, cookie=None, nextentry=None):
        self.fileid = fileid
        self.name = name
        self.cookie = cookie
        self.nextentry = nextentry

    def __repr__(self):
        out = []
        if self.fileid is not None:
            out += ['fileid=%s' % repr(self.fileid)]
        if self.name is not None:
            out += ['name=%s' % repr(self.name)]
        if self.cookie is not None:
            out += ['cookie=%s' % repr(self.cookie)]
        if self.nextentry is not None:
            out += ['nextentry=%s' % repr(self.nextentry)]
        return 'entry3(%s)' % ', '.join(out)


class dirlist3:
    # XDR definition:
    # struct dirlist3 {
    #     entry3 entries<1>;
    #     bool eof;
    # };
    def __init__(self, entries=None, eof=None):
        self.entries = entries
        self.eof = eof

    def __repr__(self):
        out = []
        if self.entries is not None:
            out += ['entries=%s' % repr(self.entries)]
        if self.eof is not None:
            out += ['eof=%s' % repr(self.eof)]
        return 'dirlist3(%s)' % ', '.join(out)


class readdir3resok:
    # XDR definition:
    # struct readdir3resok {
    #     post_op_attr dir_attributes;
    #     cookieverf3 cookieverf;
    #     dirlist3 reply;
    # };
    def __init__(self, dir_attributes=None, cookieverf=None, reply=None):
        self.dir_attributes = dir_attributes
        self.cookieverf = cookieverf
        self.reply = reply

    def __repr__(self):
        out = []
        if self.dir_attributes is not None:
            out += ['dir_attributes=%s' % repr(self.dir_attributes)]
        if self.cookieverf is not None:
            out += ['cookieverf=%s' % repr(self.cookieverf)]
        if self.reply is not None:
            out += ['reply=%s' % repr(self.reply)]
        return 'readdir3resok(%s)' % ', '.join(out)


class readdir3res:
    # XDR definition:
    # union readdir3res switch(nfsstat3 status) {
    #     case NFS3_OK:
    #         readdir3resok resok;
    #     default:
    #         post_op_attr resfail;
    # };
    def __init__(self, status=None, resok=None):
        self.status = status
        self.resok = resok

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.resok is not None:
            out += ['resok=%s' % repr(self.resok)]
        return 'readdir3res(%s)' % ', '.join(out)


class readdirplus3args:
    # XDR definition:
    # struct readdirplus3args {
    #     nfs_fh3 dir;
    #     uint64 cookie;
    #     cookieverf3 cookieverf;
    #     uint32 dircount;
    #     uint32 maxcount;
    # };
    def __init__(self, dir=None, cookie=None, cookieverf=None, dircount=None, maxcount=None):
        self.dir = dir
        self.cookie = cookie
        self.cookieverf = cookieverf
        self.dircount = dircount
        self.maxcount = maxcount

    def __repr__(self):
        out = []
        if self.dir is not None:
            out += ['dir=%s' % repr(self.dir)]
        if self.cookie is not None:
            out += ['cookie=%s' % repr(self.cookie)]
        if self.cookieverf is not None:
            out += ['cookieverf=%s' % repr(self.cookieverf)]
        if self.dircount is not None:
            out += ['dircount=%s' % repr(self.dircount)]
        if self.maxcount is not None:
            out += ['maxcount=%s' % repr(self.maxcount)]
        return 'readdirplus3args(%s)' % ', '.join(out)


class entryplus3:
    # XDR definition:
    # struct entryplus3 {
    #     uint64 fileid;
    #     filename3 name;
    #     uint64 cookie;
    #     post_op_attr name_attributes;
    #     post_op_fh3 name_handle;
    #     entryplus3 nextentry<1>;
    # };
    def __init__(self, fileid=None, name=None, cookie=None, name_attributes=None, name_handle=None, nextentry=None):
        self.fileid = fileid
        self.name = name
        self.cookie = cookie
        self.name_attributes = name_attributes
        self.name_handle = name_handle
        self.nextentry = nextentry

    def __repr__(self):
        out = []
        if self.fileid is not None:
            out += ['fileid=%s' % repr(self.fileid)]
        if self.name is not None:
            out += ['name=%s' % repr(self.name)]
        if self.cookie is not None:
            out += ['cookie=%s' % repr(self.cookie)]
        if self.name_attributes is not None:
            out += ['name_attributes=%s' % repr(self.name_attributes)]
        if self.name_handle is not None:
            out += ['name_handle=%s' % repr(self.name_handle)]
        if self.nextentry is not None:
            out += ['nextentry=%s' % repr(self.nextentry)]
        return 'entryplus3(%s)' % ', '.join(out)


class dirlistplus3:
    # XDR definition:
    # struct dirlistplus3 {
    #     entryplus3 entries<1>;
    #     bool eof;
    # };
    def __init__(self, entries=None, eof=None):
        self.entries = entries
        self.eof = eof

    def __repr__(self):
        out = []
        if self.entries is not None:
            out += ['entries=%s' % repr(self.entries)]
        if self.eof is not None:
            out += ['eof=%s' % repr(self.eof)]
        return 'dirlistplus3(%s)' % ', '.join(out)


class readdirplus3resok:
    # XDR definition:
    # struct readdirplus3resok {
    #     post_op_attr dir_attributes;
    #     cookieverf3 cookieverf;
    #     dirlistplus3 reply;
    # };
    def __init__(self, dir_attributes=None, cookieverf=None, reply=None):
        self.dir_attributes = dir_attributes
        self.cookieverf = cookieverf
        self.reply = reply

    def __repr__(self):
        out = []
        if self.dir_attributes is not None:
            out += ['dir_attributes=%s' % repr(self.dir_attributes)]
        if self.cookieverf is not None:
            out += ['cookieverf=%s' % repr(self.cookieverf)]
        if self.reply is not None:
            out += ['reply=%s' % repr(self.reply)]
        return 'readdirplus3resok(%s)' % ', '.join(out)


class readdirplus3res:
    # XDR definition:
    # union readdirplus3res switch(nfsstat3 status) {
    #     case NFS3_OK:
    #         readdirplus3resok resok;
    #     default:
    #         post_op_attr resfail;
    # };
    def __init__(self, status=None, resok=None):
        self.status = status
        self.resok = resok

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.resok is not None:
            out += ['resok=%s' % repr(self.resok)]
        return 'readdirplus3res(%s)' % ', '.join(out)


class fsstat3resok:
    # XDR definition:
    # struct fsstat3resok {
    #     post_op_attr obj_attributes;
    #     uint64 tbytes;
    #     uint64 fbytes;
    #     uint64 abytes;
    #     uint64 tfiles;
    #     uint64 ffiles;
    #     uint64 afiles;
    #     uint32 invarsec;
    # };
    def __init__(self, obj_attributes=None, tbytes=None, fbytes=None, abytes=None, tfiles=None, ffiles=None,
                 afiles=None, invarsec=None):
        self.obj_attributes = obj_attributes
        self.tbytes = tbytes
        self.fbytes = fbytes
        self.abytes = abytes
        self.tfiles = tfiles
        self.ffiles = ffiles
        self.afiles = afiles
        self.invarsec = invarsec

    def __repr__(self):
        out = []
        if self.obj_attributes is not None:
            out += ['obj_attributes=%s' % repr(self.obj_attributes)]
        if self.tbytes is not None:
            out += ['tbytes=%s' % repr(self.tbytes)]
        if self.fbytes is not None:
            out += ['fbytes=%s' % repr(self.fbytes)]
        if self.abytes is not None:
            out += ['abytes=%s' % repr(self.abytes)]
        if self.tfiles is not None:
            out += ['tfiles=%s' % repr(self.tfiles)]
        if self.ffiles is not None:
            out += ['ffiles=%s' % repr(self.ffiles)]
        if self.afiles is not None:
            out += ['afiles=%s' % repr(self.afiles)]
        if self.invarsec is not None:
            out += ['invarsec=%s' % repr(self.invarsec)]
        return 'fsstat3resok(%s)' % ', '.join(out)


class fsstat3res:
    # XDR definition:
    # union fsstat3res switch(nfsstat3 status) {
    #     case NFS3_OK:
    #         fsstat3resok resok;
    #     default:
    #         post_op_attr resfail;
    # };
    def __init__(self, status=None, resok=None):
        self.status = status
        self.resok = resok

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.resok is not None:
            out += ['resok=%s' % repr(self.resok)]
        return 'fsstat3res(%s)' % ', '.join(out)


class fsinfo3resok:
    # XDR definition:
    # struct fsinfo3resok {
    #     post_op_attr obj_attributes;
    #     uint32 rtmax;
    #     uint32 rtpref;
    #     uint32 rtmult;
    #     uint32 wtmax;
    #     uint32 wtpref;
    #     uint32 wtmult;
    #     uint32 dtpref;
    #     uint64 maxfilesize;
    #     nfstime3 time_delta;
    #     uint32 properties;
    # };
    def __init__(self, obj_attributes=None, rtmax=None, rtpref=None, rtmult=None, wtmax=None, wtpref=None,
                 wtmult=None, dtpref=None, maxfilesize=None, time_delta=None, properties=None):
        self.obj_attributes = obj_attributes
        self.rtmax = rtmax
        self.rtpref = rtpref
        self.rtmult = rtmult
        self.wtmax = wtmax
        self.wtpref = wtpref
        self.wtmult = wtmult
        self.dtpref = dtpref
        self.maxfilesize = maxfilesize
        self.time_delta = time_delta
        self.properties = properties

    def __repr__(self):
        out = []
        if self.obj_attributes is not None:
            out += ['obj_attributes=%s' % repr(self.obj_attributes)]
        if self.rtmax is not None:
            out += ['rtmax=%s' % repr(self.rtmax)]
        if self.rtpref is not None:
            out += ['rtpref=%s' % repr(self.rtpref)]
        if self.rtmult is not None:
            out += ['rtmult=%s' % repr(self.rtmult)]
        if self.wtmax is not None:
            out += ['wtmax=%s' % repr(self.wtmax)]
        if self.wtpref is not None:
            out += ['wtpref=%s' % repr(self.wtpref)]
        if self.wtmult is not None:
            out += ['wtmult=%s' % repr(self.wtmult)]
        if self.dtpref is not None:
            out += ['dtpref=%s' % repr(self.dtpref)]
        if self.maxfilesize is not None:
            out += ['maxfilesize=%s' % repr(self.maxfilesize)]
        if self.time_delta is not None:
            out += ['time_delta=%s' % repr(self.time_delta)]
        if self.properties is not None:
            out += ['properties=%s' % repr(self.properties)]
        return 'fsinfo3resok(%s)' % ', '.join(out)


class fsinfo3res:
    # XDR definition:
    # union fsinfo3res switch(nfsstat3 status) {
    #     case NFS3_OK:
    #         fsinfo3resok resok;
    #     default:
    #         post_op_attr resfail;
    # };
    def __init__(self, status=None, resok=None):
        self.status = status
        self.resok = resok

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.resok is not None:
            out += ['resok=%s' % repr(self.resok)]
        return 'fsinfo3res(%s)' % ', '.join(out)


class pathconf3resok:
    # XDR definition:
    # struct pathconf3resok {
    #     post_op_attr obj_attributes;
    #     uint32 linkmax;
    #     uint32 name_max;
    #     bool no_trunc;
    #     bool chown_restricted;
    #     bool case_insensitive;
    #     bool case_preserving;
    # };
    def __init__(self, obj_attributes=None, linkmax=None, name_max=None, no_trunc=None, chown_restricted=None,
                 case_insensitive=None, case_preserving=None):
        self.obj_attributes = obj_attributes
        self.linkmax = linkmax
        self.name_max = name_max
        self.no_trunc = no_trunc
        self.chown_restricted = chown_restricted
        self.case_insensitive = case_insensitive
        self.case_preserving = case_preserving

    def __repr__(self):
        out = []
        if self.obj_attributes is not None:
            out += ['obj_attributes=%s' % repr(self.obj_attributes)]
        if self.linkmax is not None:
            out += ['linkmax=%s' % repr(self.linkmax)]
        if self.name_max is not None:
            out += ['name_max=%s' % repr(self.name_max)]
        if self.no_trunc is not None:
            out += ['no_trunc=%s' % repr(self.no_trunc)]
        if self.chown_restricted is not None:
            out += ['chown_restricted=%s' % repr(self.chown_restricted)]
        if self.case_insensitive is not None:
            out += ['case_insensitive=%s' % repr(self.case_insensitive)]
        if self.case_preserving is not None:
            out += ['case_preserving=%s' % repr(self.case_preserving)]
        return 'pathconf3resok(%s)' % ', '.join(out)


class pathconf3res:
    # XDR definition:
    # union pathconf3res switch(nfsstat3 status) {
    #     case NFS3_OK:
    #         pathconf3resok resok;
    #     default:
    #         post_op_attr resfail;
    # };
    def __init__(self, status=None, resok=None):
        self.status = status
        self.resok = resok

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.resok is not None:
            out += ['resok=%s' % repr(self.resok)]
        return 'pathconf3res(%s)' % ', '.join(out)


class commit3args:
    # XDR definition:
    # struct commit3args {
    #     nfs_fh3 file;
    #     uint64 offset;
    #     uint32 count;
    # };
    def __init__(self, file=None, offset=None, count=None):
        self.file = file
        self.offset = offset
        self.count = count

    def __repr__(self):
        out = []
        if self.file is not None:
            out += ['file=%s' % repr(self.file)]
        if self.offset is not None:
            out += ['offset=%s' % repr(self.offset)]
        if self.count is not None:
            out += ['count=%s' % repr(self.count)]
        return 'commit3args(%s)' % ', '.join(out)


class commit3resok:
    # XDR definition:
    # struct commit3resok {
    #     wcc_data file_wcc;
    #     writeverf3 verf;
    # };
    def __init__(self, file_wcc=None, verf=None):
        self.file_wcc = file_wcc
        self.verf = verf

    def __repr__(self):
        out = []
        if self.file_wcc is not None:
            out += ['file_wcc=%s' % repr(self.file_wcc)]
        if self.verf is not None:
            out += ['verf=%s' % repr(self.verf)]
        return 'commit3resok(%s)' % ', '.join(out)


class commit3res:
    # XDR definition:
    # union commit3res switch(nfsstat3 status) {
    #     case NFS3_OK:
    #         commit3resok resok;
    #     default:
    #         wcc_data resfail;
    # };
    def __init__(self, status=None, resok=None):
        self.status = status
        self.resok = resok

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['status=%s' % const.NFSSTAT3.get(self.status, self.status)]
        if self.resok is not None:
            out += ['resok=%s' % repr(self.resok)]
        return 'commit3res(%s)' % ', '.join(out)


class setaclargs:
    # XDR definition:
    # struct setaclargs {
    #     diropargs3 dargs;
    #     write3args wargs;
    # };
    def __init__(self, dargs=None, wargs=None):
        self.dargs = dargs
        self.wargs = wargs

    def __repr__(self):
        out = []
        if self.dargs is not None:
            out += ['dargs=%s' % repr(self.dargs)]
        if self.wargs is not None:
            out += ['wargs=%s' % repr(self.wargs)]
        return 'setaclargs(%s)' % ', '.join(out)


class mountres3_ok:
    # XDR definition:
    # struct mountres3_ok {
    #     fhandle3 fhandle;
    #     int auth_flavors<>;
    # };
    def __init__(self, fhandle=None, auth_flavors=None):
        self.fhandle = fhandle
        self.auth_flavors = auth_flavors

    def __repr__(self):
        out = []
        if self.fhandle is not None:
            out += ['fhandle=%s' % repr(self.fhandle)]
        if self.auth_flavors is not None:
            out += ['auth_flavors=%s' % repr(self.auth_flavors)]
        return 'mountres3_ok(%s)' % ', '.join(out)


class mountres3:
    # XDR definition:
    # union mountres3 switch(mountstat3 fhs_status) {
    #     case MNT3_OK:
    #         mountres3_ok mountinfo;
    #     default:
    #         void;
    # };
    def __init__(self, fhs_status=None, mountinfo=None):
        self.status = fhs_status
        self.mountinfo = mountinfo

    def __repr__(self):
        out = []
        if self.status is not None:
            out += ['fhs_status=%s' % const.MOUNTSTAT3.get(self.status, self.status)]
        if self.mountinfo is not None:
            out += ['mountinfo=%s' % repr(self.mountinfo)]
        return 'mountres3(%s)' % ', '.join(out)


class mountbody:
    # XDR definition:
    # struct mountbody {
    #     name ml_hostname;
    #     dirpath ml_directory;
    #     mountlist ml_next;
    # };
    def __init__(self, ml_hostname=None, ml_directory=None, ml_next=None):
        self.ml_hostname = ml_hostname
        self.ml_directory = ml_directory
        self.ml_next = ml_next

    def __repr__(self):
        out = []
        if self.ml_hostname is not None:
            out += ['ml_hostname=%s' % repr(self.ml_hostname)]
        if self.ml_directory is not None:
            out += ['ml_directory=%s' % repr(self.ml_directory)]
        if self.ml_next is not None:
            out += ['ml_next=%s' % repr(self.ml_next)]
        return 'mountbody(%s)' % ', '.join(out)


class groupnode:
    # XDR definition:
    # struct groupnode {
    #     name gr_name;
    #     groups gr_next;
    # };
    def __init__(self, gr_name=None, gr_next=None):
        self.gr_name = gr_name
        self.gr_next = gr_next

    def __repr__(self):
        out = []
        if self.gr_name is not None:
            out += ['gr_name=%s' % repr(self.gr_name)]
        if self.gr_next is not None:
            out += ['gr_next=%s' % repr(self.gr_next)]
        return 'groupnode(%s)' % ', '.join(out)


class exportnode:
    # XDR definition:
    # struct exportnode {
    #     dirpath ex_dir;
    #     groups ex_groups;
    #     exports ex_next;
    # };
    def __init__(self, ex_dir=None, ex_groups=None, ex_next=None):
        self.ex_dir = ex_dir
        self.ex_groups = ex_groups
        self.ex_next = ex_next

    def __repr__(self):
        out = []
        if self.ex_dir is not None:
            out += ['ex_dir=%s' % repr(self.ex_dir)]
        if self.ex_groups is not None:
            out += ['ex_groups=%s' % repr(self.ex_groups)]
        if self.ex_next is not None:
            out += ['ex_next=%s' % repr(self.ex_next)]
        return 'exportnode(%s)' % ', '.join(out)
