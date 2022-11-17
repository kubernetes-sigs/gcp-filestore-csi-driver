import struct
from xdrlib import Packer, Unpacker, ConversionError
from xdrlib import Error as XDRError
from . import const
from . import rtypes as types


class nullclass(object):
    pass


class nfs_pro_v3Packer(Packer):
    pack_hyper = Packer.pack_hyper
    pack_string = Packer.pack_string
    pack_int = Packer.pack_int
    pack_float = Packer.pack_float
    pack_uint = Packer.pack_uint
    pack_opaque = Packer.pack_opaque
    pack_double = Packer.pack_double
    pack_unsigned = Packer.pack_uint
    pack_quadruple = Packer.pack_double
    pack_uhyper = Packer.pack_uhyper
    pack_bool = Packer.pack_bool
    pack_uint32 = pack_uint64 = pack_uint

    def pack_uint64(self, x):
        try:
            self._Packer__buf.write(struct.pack('>Q', x))
        except struct.error as e:
            raise ConversionError(e.args[0])

    def pack_filename3(self, data):
        self.pack_string(data)

    def pack_nfspath3(self, data):
        self.pack_string(data)

    def pack_cookieverf3(self, data):
        self.pack_fopaque(const.NFS3_COOKIEVERFSIZE, data)

    def pack_createverf3(self, data):
        self.pack_fopaque(const.NFS3_CREATEVERFSIZE, data)

    def pack_writeverf3(self, data):
        self.pack_fopaque(const.NFS3_WRITEVERFSIZE, data)

    def pack_nfsstat3(self, data):
        if data not in [const.NFS3_OK, const.NFS3ERR_PERM, const.NFS3ERR_NOENT, const.NFS3ERR_IO, const.NFS3ERR_NXIO,
                        const.NFS3ERR_ACCES, const.NFS3ERR_EXIST, const.NFS3ERR_XDEV, const.NFS3ERR_NODEV,
                        const.NFS3ERR_NOTDIR, const.NFS3ERR_ISDIR, const.NFS3ERR_INVAL, const.NFS3ERR_FBIG,
                        const.NFS3ERR_NOSPC, const.NFS3ERR_ROFS, const.NFS3ERR_MLINK, const.NFS3ERR_NAMETOOLONG,
                        const.NFS3ERR_NOTEMPTY, const.NFS3ERR_DQUOT, const.NFS3ERR_STALE, const.NFS3ERR_REMOTE,
                        const.NFS3ERR_BADHANDLE, const.NFS3ERR_NOT_SYNC, const.NFS3ERR_BAD_COOKIE,
                        const.NFS3ERR_NOTSUPP, const.NFS3ERR_TOOSMALL, const.NFS3ERR_SERVERFAULT, const.NFS3ERR_BADTYPE,
                        const.NFS3ERR_JUKEBOX]:
            raise XDRError('value=%s not in enum nfsstat3' % data)
        self.pack_int(data)

    def pack_ftype3(self, data):
        if data not in [const.NF3REG, const.NF3DIR, const.NF3BLK, const.NF3CHR, const.NF3LNK, const.NF3SOCK,
                        const.NF3FIFO]:
            raise XDRError('value=%s not in enum ftype3' % data)
        self.pack_int(data)

    def pack_specdata3(self, data):
        if data.major is None:
            raise TypeError('data.major == None')
        self.pack_uint32(data.major)
        if data.minor is None:
            raise TypeError('data.minor == None')
        self.pack_uint32(data.minor)

    def pack_nfs_fh3(self, data):
        if data.data is None:
            raise TypeError('data.data == None')
        if len(data.data) > const.NFS3_FHSIZE:
            raise XDRError('array length too long for data.data')
        self.pack_opaque(data.data)

    def pack_nfstime3(self, data):
        if data.seconds is None:
            raise TypeError('data.seconds == None')
        self.pack_uint32(data.seconds)
        if data.nseconds is None:
            raise TypeError('data.nseconds == None')
        self.pack_uint32(data.nseconds)

    def pack_fattr3(self, data):
        if data.type is None:
            raise TypeError('data.type == None')
        self.pack_ftype3(data.type)
        if data.mode is None:
            raise TypeError('data.mode == None')
        self.pack_uint32(data.mode)
        if data.nlink is None:
            raise TypeError('data.nlink == None')
        self.pack_uint32(data.nlink)
        if data.uid is None:
            raise TypeError('data.uid == None')
        self.pack_uint32(data.uid)
        if data.gid is None:
            raise TypeError('data.gid == None')
        self.pack_uint32(data.gid)
        if data.size is None:
            raise TypeError('data.size == None')
        self.pack_uint64(data.size)
        if data.used is None:
            raise TypeError('data.used == None')
        self.pack_uint64(data.used)
        if data.rdev is None:
            raise TypeError('data.rdev == None')
        self.pack_specdata3(data.rdev)
        if data.fsid is None:
            raise TypeError('data.fsid == None')
        self.pack_uint64(data.fsid)
        if data.fileid is None:
            raise TypeError('data.fileid == None')
        self.pack_uint64(data.fileid)
        if data.atime is None:
            raise TypeError('data.atime == None')
        self.pack_nfstime3(data.atime)
        if data.mtime is None:
            raise TypeError('data.mtime == None')
        self.pack_nfstime3(data.mtime)
        if data.ctime is None:
            raise TypeError('data.ctime == None')
        self.pack_nfstime3(data.ctime)

    def pack_post_op_attr(self, data):
        if data.present is None:
            raise TypeError('data.present == None')
        self.pack_bool(data.present)
        if data.present == const.TRUE:
            if data.attributes is None:
                raise TypeError('data.attributes == None')
            self.pack_fattr3(data.attributes)
        elif data.present == const.FALSE:
            pass
        else:
            raise XDRError('bad switch=%s' % data.present)

    def pack_wcc_attr(self, data):
        if data.size is None:
            raise TypeError('data.size == None')
        self.pack_uint64(data.size)
        if data.mtime is None:
            raise TypeError('data.mtime == None')
        self.pack_nfstime3(data.mtime)
        if data.ctime is None:
            raise TypeError('data.ctime == None')
        self.pack_nfstime3(data.ctime)

    def pack_pre_op_attr(self, data):
        if data.present is None:
            raise TypeError('data.present == None')
        self.pack_bool(data.present)
        if data.present == const.TRUE:
            if data.attributes is None:
                raise TypeError('data.attributes == None')
            self.pack_wcc_attr(data.attributes)
        elif data.present == const.FALSE:
            pass
        else:
            raise XDRError('bad switch=%s' % data.present)

    def pack_wcc_data(self, data):
        if data.before is None:
            raise TypeError('data.before == None')
        self.pack_pre_op_attr(data.before)
        if data.after is None:
            raise TypeError('data.after == None')
        self.pack_post_op_attr(data.after)

    def pack_post_op_fh3(self, data):
        if data.present is None:
            raise TypeError('data.present == None')
        self.pack_bool(data.present)
        if data.present == const.TRUE:
            if data.handle is None:
                raise TypeError('data.handle == None')
            self.pack_nfs_fh3(data.handle)
        elif data.present == const.FALSE:
            pass
        else:
            raise XDRError('bad switch=%s' % data.present)

    def pack_set_uint32(self, data):
        if data.set is None:
            raise TypeError('data.set == None')
        self.pack_bool(data.set)
        if data.set == const.TRUE:
            if data.val is None:
                raise TypeError('data.val == None')
            self.pack_uint32(data.val)
        else:
            pass

    def pack_set_uint64(self, data):
        if data.set is None:
            raise TypeError('data.set == None')
        self.pack_bool(data.set)
        if data.set == const.TRUE:
            if data.val is None:
                raise TypeError('data.val == None')
            self.pack_uint64(data.val)
        else:
            pass

    def pack_time_how(self, data):
        if data not in [const.DONT_CHANGE, const.SET_TO_SERVER_TIME, const.SET_TO_CLIENT_TIME]:
            raise XDRError('value=%s not in enum time_how' % data)
        self.pack_int(data)

    def pack_set_time(self, data):
        if data.set is None:
            raise TypeError('data.set == None')
        self.pack_time_how(data.set)
        if data.set == const.SET_TO_CLIENT_TIME:
            if data.time is None:
                raise TypeError('data.time == None')
            self.pack_nfstime3(data.time)
        else:
            pass

    def pack_sattr3(self, data):
        if data.mode is None:
            raise TypeError('data.mode == None')
        self.pack_set_uint32(data.mode)
        if data.uid is None:
            raise TypeError('data.uid == None')
        self.pack_set_uint32(data.uid)
        if data.gid is None:
            raise TypeError('data.gid == None')
        self.pack_set_uint32(data.gid)
        if data.size is None:
            raise TypeError('data.size == None')
        self.pack_set_uint64(data.size)
        if data.atime is None:
            raise TypeError('data.atime == None')
        self.pack_set_time(data.atime)
        if data.mtime is None:
            raise TypeError('data.mtime == None')
        self.pack_set_time(data.mtime)

    def pack_diropargs3(self, data):
        if data.dir is None:
            raise TypeError('data.dir == None')
        self.pack_nfs_fh3(data.dir)
        if data.name is None:
            raise TypeError('data.name == None')
        self.pack_filename3(data.name)

    def pack_diropres3ok(self, data):
        if data.obj is None:
            raise TypeError('data.obj == None')
        self.pack_post_op_fh3(data.obj)
        if data.obj_attributes is None:
            raise TypeError('data.obj_attributes == None')
        self.pack_post_op_attr(data.obj_attributes)
        if data.dir_wcc is None:
            raise TypeError('data.dir_wcc == None')
        self.pack_wcc_data(data.dir_wcc)

    def pack_diropres3(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == const.NFS3_OK:
            if data.resok is None:
                raise TypeError('data.resok == None')
            self.pack_diropres3ok(data.resok)
        else:
            if data.resfail is None:
                raise TypeError('data.resfail == None')
            self.pack_wcc_data(data.resfail)

    def pack_wccstat3(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == -1:
            pass
        else:
            if data.wcc is None:
                raise TypeError('data.wcc == None')
            self.pack_wcc_data(data.wcc)

    def pack_getattr3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == const.NFS3_OK:
            if data.attributes is None:
                raise TypeError('data.attributes == None')
            self.pack_fattr3(data.attributes)
        else:
            pass

    def pack_sattrguard3(self, data):
        if data.check is None:
            raise TypeError('data.check == None')
        self.pack_bool(data.check)
        if data.check == const.TRUE:
            if data.ctime is None:
                raise TypeError('data.ctime == None')
            self.pack_nfstime3(data.ctime)
        elif data.check == const.FALSE:
            pass
        else:
            raise XDRError('bad switch=%s' % data.check)

    def pack_setattr3args(self, data):
        if data.object is None:
            raise TypeError('data.object == None')
        self.pack_nfs_fh3(data.object)
        if data.new_attributes is None:
            raise TypeError('data.new_attributes == None')
        self.pack_sattr3(data.new_attributes)
        if data.guard is None:
            raise TypeError('data.guard == None')
        self.pack_sattrguard3(data.guard)

    def pack_lookup3resok(self, data):
        if data.object is None:
            raise TypeError('data.object == None')
        self.pack_nfs_fh3(data.object)
        if data.obj_attributes is None:
            raise TypeError('data.obj_attributes == None')
        self.pack_post_op_attr(data.obj_attributes)
        if data.dir_attributes is None:
            raise TypeError('data.dir_attributes == None')
        self.pack_post_op_attr(data.dir_attributes)

    def pack_lookup3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == const.NFS3_OK:
            if data.resok is None:
                raise TypeError('data.resok == None')
            self.pack_lookup3resok(data.resok)
        else:
            if data.resfail is None:
                raise TypeError('data.resfail == None')
            self.pack_post_op_attr(data.resfail)

    def pack_access3args(self, data):
        if data.object is None:
            raise TypeError('data.object == None')
        self.pack_nfs_fh3(data.object)
        if data.access is None:
            raise TypeError('data.access == None')
        self.pack_uint32(data.access)

    def pack_access3resok(self, data):
        if data.obj_attributes is None:
            raise TypeError('data.obj_attributes == None')
        self.pack_post_op_attr(data.obj_attributes)
        if data.access is None:
            raise TypeError('data.access == None')
        self.pack_uint32(data.access)

    def pack_access3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == const.NFS3_OK:
            if data.resok is None:
                raise TypeError('data.resok == None')
            self.pack_access3resok(data.resok)
        else:
            if data.resfail is None:
                raise TypeError('data.resfail == None')
            self.pack_post_op_attr(data.resfail)

    def pack_readlink3resok(self, data):
        if data.symlink_attributes is None:
            raise TypeError('data.symlink_attributes == None')
        self.pack_post_op_attr(data.symlink_attributes)
        if data.data is None:
            raise TypeError('data.data == None')
        self.pack_nfspath3(data.data)

    def pack_readlink3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == const.NFS3_OK:
            if data.resok is None:
                raise TypeError('data.resok == None')
            self.pack_readlink3resok(data.resok)
        else:
            if data.resfail is None:
                raise TypeError('data.resfail == None')
            self.pack_post_op_attr(data.resfail)

    def pack_read3args(self, data):
        if data.file is None:
            raise TypeError('data.file == None')
        self.pack_nfs_fh3(data.file)
        if data.offset is None:
            raise TypeError('data.offset == None')
        self.pack_uint64(data.offset)
        if data.count is None:
            raise TypeError('data.count == None')
        self.pack_uint32(data.count)

    def pack_read3resok(self, data):
        if data.file_attributes is None:
            raise TypeError('data.file_attributes == None')
        self.pack_post_op_attr(data.file_attributes)
        if data.count is None:
            raise TypeError('data.count == None')
        self.pack_uint32(data.count)
        if data.eof is None:
            raise TypeError('data.eof == None')
        self.pack_bool(data.eof)
        if data.data is None:
            raise TypeError('data.data == None')
        self.pack_opaque(data.data)

    def pack_read3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == const.NFS3_OK:
            if data.resok is None:
                raise TypeError('data.resok == None')
            self.pack_read3resok(data.resok)
        else:
            if data.resfail is None:
                raise TypeError('data.resfail == None')
            self.pack_post_op_attr(data.resfail)

    def pack_stable_how(self, data):
        if data not in [const.UNSTABLE, const.DATA_SYNC, const.FILE_SYNC]:
            raise XDRError('value=%s not in enum stable_how' % data)
        self.pack_int(data)

    def pack_write3args(self, data):
        if data.file is None:
            raise TypeError('data.file == None')
        self.pack_nfs_fh3(data.file)
        if data.offset is None:
            raise TypeError('data.offset == None')
        self.pack_uint64(data.offset)
        if data.count is None:
            raise TypeError('data.count == None')
        self.pack_uint32(data.count)
        if data.stable is None:
            raise TypeError('data.stable == None')
        self.pack_stable_how(data.stable)
        if data.data is None:
            raise TypeError('data.data == None')
        self.pack_opaque(data.data)

    def pack_write3resok(self, data):
        if data.file_wcc is None:
            raise TypeError('data.file_wcc == None')
        self.pack_wcc_data(data.file_wcc)
        if data.count is None:
            raise TypeError('data.count == None')
        self.pack_uint32(data.count)
        if data.committed is None:
            raise TypeError('data.committed == None')
        self.pack_stable_how(data.committed)
        if data.verf is None:
            raise TypeError('data.verf == None')
        self.pack_writeverf3(data.verf)

    def pack_write3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == const.NFS3_OK:
            if data.resok is None:
                raise TypeError('data.resok == None')
            self.pack_write3resok(data.resok)
        else:
            if data.resfail is None:
                raise TypeError('data.resfail == None')
            self.pack_wcc_data(data.resfail)

    def pack_createmode3(self, data):
        if data not in [const.UNCHECKED, const.GUARDED, const.EXCLUSIVE]:
            raise XDRError('value=%s not in enum createmode3' % data)
        self.pack_int(data)

    def pack_createhow3(self, data):
        if data.mode is None:
            raise TypeError('data.mode == None')
        self.pack_createmode3(data.mode)
        if data.mode == const.UNCHECKED or data.mode == const.GUARDED:
            if data.obj_attributes is None:
                raise TypeError('data.obj_attributes == None')
            self.pack_sattr3(data.obj_attributes)
        elif data.mode == const.EXCLUSIVE:
            if data.verf is None:
                raise TypeError('data.verf == None')
            self.pack_createverf3(data.verf)
        else:
            raise XDRError('bad switch=%s' % data.mode)

    def pack_create3args(self, data):
        if data.where is None:
            raise TypeError('data.where == None')
        self.pack_diropargs3(data.where)
        if data.how is None:
            raise TypeError('data.how == None')
        self.pack_createhow3(data.how)

    def pack_mkdir3args(self, data):
        if data.where is None:
            raise TypeError('data.where == None')
        self.pack_diropargs3(data.where)
        if data.attributes is None:
            raise TypeError('data.attributes == None')
        self.pack_sattr3(data.attributes)

    def pack_symlinkdata3(self, data):
        if data.symlink_attributes is None:
            raise TypeError('data.symlink_attributes == None')
        self.pack_sattr3(data.symlink_attributes)
        if data.symlink_data is None:
            raise TypeError('data.symlink_data == None')
        self.pack_nfspath3(data.symlink_data)

    def pack_symlink3args(self, data):
        if data.where is None:
            raise TypeError('data.where == None')
        self.pack_diropargs3(data.where)
        if data.symlink is None:
            raise TypeError('data.symlink == None')
        self.pack_symlinkdata3(data.symlink)

    def pack_devicedata3(self, data):
        if data.dev_attributes is None:
            raise TypeError('data.dev_attributes == None')
        self.pack_sattr3(data.dev_attributes)
        if data.spec is None:
            raise TypeError('data.spec == None')
        self.pack_specdata3(data.spec)

    def pack_mknoddata3(self, data):
        if data.type is None:
            raise TypeError('data.type == None')
        self.pack_ftype3(data.type)
        if data.type == const.NF3CHR or data.type == const.NF3BLK:
            if data.device is None:
                raise TypeError('data.device == None')
            self.pack_devicedata3(data.device)
        elif data.type == const.NF3SOCK or data.type == const.NF3FIFO:
            if data.pipe_attributes is None:
                raise TypeError('data.pipe_attributes == None')
            self.pack_sattr3(data.pipe_attributes)
        else:
            pass

    def pack_mknod3args(self, data):
        if data.where is None:
            raise TypeError('data.where == None')
        self.pack_diropargs3(data.where)
        if data.what is None:
            raise TypeError('data.what == None')
        self.pack_mknoddata3(data.what)

    def pack_rename3args(self, data):
        if data.from_v is None:
            raise TypeError('data.from == None')
        self.pack_diropargs3(data.from_v)
        if data.to is None:
            raise TypeError('data.to == None')
        self.pack_diropargs3(data.to)

    def pack_rename3wcc(self, data):
        if data.fromdir_wcc is None:
            raise TypeError('data.fromdir_wcc == None')
        self.pack_wcc_data(data.fromdir_wcc)
        if data.todir_wcc is None:
            raise TypeError('data.todir_wcc == None')
        self.pack_wcc_data(data.todir_wcc)

    def pack_rename3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == -1:
            pass
        else:
            if data.res is None:
                raise TypeError('data.res == None')
            self.pack_rename3wcc(data.res)

    def pack_link3args(self, data):
        if data.file is None:
            raise TypeError('data.file == None')
        self.pack_nfs_fh3(data.file)
        if data.link is None:
            raise TypeError('data.link == None')
        self.pack_diropargs3(data.link)

    def pack_link3wcc(self, data):
        if data.file_attributes is None:
            raise TypeError('data.file_attributes == None')
        self.pack_post_op_attr(data.file_attributes)
        if data.linkdir_wcc is None:
            raise TypeError('data.linkdir_wcc == None')
        self.pack_wcc_data(data.linkdir_wcc)

    def pack_link3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == -1:
            pass
        else:
            if data.res is None:
                raise TypeError('data.res == None')
            self.pack_link3wcc(data.res)

    def pack_readdir3args(self, data):
        if data.dir is None:
            raise TypeError('data.dir == None')
        self.pack_nfs_fh3(data.dir)
        if data.cookie is None:
            raise TypeError('data.cookie == None')
        self.pack_uint64(data.cookie)
        if data.cookieverf is None:
            raise TypeError('data.cookieverf == None')
        self.pack_cookieverf3(data.cookieverf)
        if data.count is None:
            raise TypeError('data.count == None')
        self.pack_uint32(data.count)

    def pack_entry3(self, data):
        if data.fileid is None:
            raise TypeError('data.fileid == None')
        self.pack_uint64(data.fileid)
        if data.name is None:
            raise TypeError('data.name == None')
        self.pack_filename3(data.name)
        if data.cookie is None:
            raise TypeError('data.cookie == None')
        self.pack_uint64(data.cookie)
        if data.nextentry is None:
            raise TypeError('data.nextentry == None')
        if len(data.nextentry) > 1:
            raise XDRError('array length too long for data.nextentry')
        self.pack_array(data.nextentry, self.pack_entry3)

    def pack_dirlist3(self, data):
        if data.entries is None:
            raise TypeError('data.entries == None')
        if len(data.entries) > 1:
            raise XDRError('array length too long for data.entries')
        self.pack_array(data.entries, self.pack_entry3)
        if data.eof is None:
            raise TypeError('data.eof == None')
        self.pack_bool(data.eof)

    def pack_readdir3resok(self, data):
        if data.dir_attributes is None:
            raise TypeError('data.dir_attributes == None')
        self.pack_post_op_attr(data.dir_attributes)
        if data.cookieverf is None:
            raise TypeError('data.cookieverf == None')
        self.pack_cookieverf3(data.cookieverf)
        if data.reply is None:
            raise TypeError('data.reply == None')
        self.pack_dirlist3(data.reply)

    def pack_readdir3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == const.NFS3_OK:
            if data.resok is None:
                raise TypeError('data.resok == None')
            self.pack_readdir3resok(data.resok)
        else:
            if data.resfail is None:
                raise TypeError('data.resfail == None')
            self.pack_post_op_attr(data.resfail)

    def pack_readdirplus3args(self, data):
        if data.dir is None:
            raise TypeError('data.dir == None')
        self.pack_nfs_fh3(data.dir)
        if data.cookie is None:
            raise TypeError('data.cookie == None')
        self.pack_uint64(data.cookie)
        if data.cookieverf is None:
            raise TypeError('data.cookieverf == None')
        self.pack_cookieverf3(data.cookieverf)
        if data.dircount is None:
            raise TypeError('data.dircount == None')
        self.pack_uint32(data.dircount)
        if data.maxcount is None:
            raise TypeError('data.maxcount == None')
        self.pack_uint32(data.maxcount)

    def pack_entryplus3(self, data):
        if data.fileid is None:
            raise TypeError('data.fileid == None')
        self.pack_uint64(data.fileid)
        if data.name is None:
            raise TypeError('data.name == None')
        self.pack_filename3(data.name)
        if data.cookie is None:
            raise TypeError('data.cookie == None')
        self.pack_uint64(data.cookie)
        if data.name_attributes is None:
            raise TypeError('data.name_attributes == None')
        self.pack_post_op_attr(data.name_attributes)
        if data.name_handle is None:
            raise TypeError('data.name_handle == None')
        self.pack_post_op_fh3(data.name_handle)
        if data.nextentry is None:
            raise TypeError('data.nextentry == None')
        if len(data.nextentry) > 1:
            raise XDRError('array length too long for data.nextentry')
        self.pack_array(data.nextentry, self.pack_entryplus3)

    def pack_dirlistplus3(self, data):
        if data.entries is None:
            raise TypeError('data.entries == None')
        if len(data.entries) > 1:
            raise XDRError('array length too long for data.entries')
        self.pack_array(data.entries, self.pack_entryplus3)
        if data.eof is None:
            raise TypeError('data.eof == None')
        self.pack_bool(data.eof)

    def pack_readdirplus3resok(self, data):
        if data.dir_attributes is None:
            raise TypeError('data.dir_attributes == None')
        self.pack_post_op_attr(data.dir_attributes)
        if data.cookieverf is None:
            raise TypeError('data.cookieverf == None')
        self.pack_cookieverf3(data.cookieverf)
        if data.reply is None:
            raise TypeError('data.reply == None')
        self.pack_dirlistplus3(data.reply)

    def pack_readdirplus3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == const.NFS3_OK:
            if data.resok is None:
                raise TypeError('data.resok == None')
            self.pack_readdirplus3resok(data.resok)
        else:
            if data.resfail is None:
                raise TypeError('data.resfail == None')
            self.pack_post_op_attr(data.resfail)

    def pack_fsstat3resok(self, data):
        if data.obj_attributes is None:
            raise TypeError('data.obj_attributes == None')
        self.pack_post_op_attr(data.obj_attributes)
        if data.tbytes is None:
            raise TypeError('data.tbytes == None')
        self.pack_uint64(data.tbytes)
        if data.fbytes is None:
            raise TypeError('data.fbytes == None')
        self.pack_uint64(data.fbytes)
        if data.abytes is None:
            raise TypeError('data.abytes == None')
        self.pack_uint64(data.abytes)
        if data.tfiles is None:
            raise TypeError('data.tfiles == None')
        self.pack_uint64(data.tfiles)
        if data.ffiles is None:
            raise TypeError('data.ffiles == None')
        self.pack_uint64(data.ffiles)
        if data.afiles is None:
            raise TypeError('data.afiles == None')
        self.pack_uint64(data.afiles)
        if data.invarsec is None:
            raise TypeError('data.invarsec == None')
        self.pack_uint32(data.invarsec)

    def pack_fsstat3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == const.NFS3_OK:
            if data.resok is None:
                raise TypeError('data.resok == None')
            self.pack_fsstat3resok(data.resok)
        else:
            if data.resfail is None:
                raise TypeError('data.resfail == None')
            self.pack_post_op_attr(data.resfail)

    def pack_fsinfo3resok(self, data):
        if data.obj_attributes is None:
            raise TypeError('data.obj_attributes == None')
        self.pack_post_op_attr(data.obj_attributes)
        if data.rtmax is None:
            raise TypeError('data.rtmax == None')
        self.pack_uint32(data.rtmax)
        if data.rtpref is None:
            raise TypeError('data.rtpref == None')
        self.pack_uint32(data.rtpref)
        if data.rtmult is None:
            raise TypeError('data.rtmult == None')
        self.pack_uint32(data.rtmult)
        if data.wtmax is None:
            raise TypeError('data.wtmax == None')
        self.pack_uint32(data.wtmax)
        if data.wtpref is None:
            raise TypeError('data.wtpref == None')
        self.pack_uint32(data.wtpref)
        if data.wtmult is None:
            raise TypeError('data.wtmult == None')
        self.pack_uint32(data.wtmult)
        if data.dtpref is None:
            raise TypeError('data.dtpref == None')
        self.pack_uint32(data.dtpref)
        if data.maxfilesize is None:
            raise TypeError('data.maxfilesize == None')
        self.pack_uint64(data.maxfilesize)
        if data.time_delta is None:
            raise TypeError('data.time_delta == None')
        self.pack_nfstime3(data.time_delta)
        if data.properties is None:
            raise TypeError('data.properties == None')
        self.pack_uint32(data.properties)

    def pack_fsinfo3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == const.NFS3_OK:
            if data.resok is None:
                raise TypeError('data.resok == None')
            self.pack_fsinfo3resok(data.resok)
        else:
            if data.resfail is None:
                raise TypeError('data.resfail == None')
            self.pack_post_op_attr(data.resfail)

    def pack_pathconf3resok(self, data):
        if data.obj_attributes is None:
            raise TypeError('data.obj_attributes == None')
        self.pack_post_op_attr(data.obj_attributes)
        if data.linkmax is None:
            raise TypeError('data.linkmax == None')
        self.pack_uint32(data.linkmax)
        if data.name_max is None:
            raise TypeError('data.name_max == None')
        self.pack_uint32(data.name_max)
        if data.no_trunc is None:
            raise TypeError('data.no_trunc == None')
        self.pack_bool(data.no_trunc)
        if data.chown_restricted is None:
            raise TypeError('data.chown_restricted == None')
        self.pack_bool(data.chown_restricted)
        if data.case_insensitive is None:
            raise TypeError('data.case_insensitive == None')
        self.pack_bool(data.case_insensitive)
        if data.case_preserving is None:
            raise TypeError('data.case_preserving == None')
        self.pack_bool(data.case_preserving)

    def pack_pathconf3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == const.NFS3_OK:
            if data.resok is None:
                raise TypeError('data.resok == None')
            self.pack_pathconf3resok(data.resok)
        else:
            if data.resfail is None:
                raise TypeError('data.resfail == None')
            self.pack_post_op_attr(data.resfail)

    def pack_commit3args(self, data):
        if data.file is None:
            raise TypeError('data.file == None')
        self.pack_nfs_fh3(data.file)
        if data.offset is None:
            raise TypeError('data.offset == None')
        self.pack_uint64(data.offset)
        if data.count is None:
            raise TypeError('data.count == None')
        self.pack_uint32(data.count)

    def pack_commit3resok(self, data):
        if data.file_wcc is None:
            raise TypeError('data.file_wcc == None')
        self.pack_wcc_data(data.file_wcc)
        if data.verf is None:
            raise TypeError('data.verf == None')
        self.pack_writeverf3(data.verf)

    def pack_commit3res(self, data):
        if data.status is None:
            raise TypeError('data.status == None')
        self.pack_nfsstat3(data.status)
        if data.status == const.NFS3_OK:
            if data.resok is None:
                raise TypeError('data.resok == None')
            self.pack_commit3resok(data.resok)
        else:
            if data.resfail is None:
                raise TypeError('data.resfail == None')
            self.pack_wcc_data(data.resfail)

    def pack_setaclargs(self, data):
        if data.dargs is None:
            raise TypeError('data.dargs == None')
        self.pack_diropargs3(data.dargs)
        if data.wargs is None:
            raise TypeError('data.wargs == None')
        self.pack_write3args(data.wargs)

    def pack_dirpath(self, data):
        if len(data) > const.NFS3_MNTPATHLEN:
            raise XDRError('array length too long for data')
        self.pack_string(data)

    def pack_name(self, data):
        if len(data) > const.NFS3_MNTNAMLEN:
            raise XDRError('array length too long for data')
        self.pack_string(data)

    def pack_fhandle3(self, data):
        if len(data) > const.NFS3_FHSIZE:
            raise XDRError('array length too long for data')
        self.pack_opaque(data)

    def pack_mountstat3(self, data):
        if data not in [const.MNT3_OK, const.MNT3ERR_PERM, const.MNT3ERR_NOENT, const.MNT3ERR_IO, const.MNT3ERR_ACCES,
                        const.MNT3ERR_NOTDIR, const.MNT3ERR_INVAL, const.MNT3ERR_NAMETOOLONG, const.MNT3ERR_NOTSUPP,
                        const.MNT3ERR_SERVERFAULT]:
            raise XDRError('value=%s not in enum mountstat3' % data)
        self.pack_int(data)

    def pack_mountres3_ok(self, data):
        if data.fhandle is None:
            raise TypeError('data.fhandle == None')
        self.pack_fhandle3(data.fhandle)
        if data.auth_flavors is None:
            raise TypeError('data.auth_flavors == None')
        self.pack_array(data.auth_flavors, self.pack_int)

    def pack_mountres3(self, data):
        if data.fhs_status is None:
            raise TypeError('data.fhs_status == None')
        self.pack_mountstat3(data.fhs_status)
        if data.fhs_status == const.MNT3_OK:
            if data.mountinfo is None:
                raise TypeError('data.mountinfo == None')
            self.pack_mountres3_ok(data.mountinfo)
        else:
            pass

    def pack_mountlist(self, data):
        if len(data) > 1:
            raise XDRError('array length too long for data')
        self.pack_array(data, self.pack_mountbody)

    def pack_mountbody(self, data):
        if data.ml_hostname is None:
            raise TypeError('data.ml_hostname == None')
        self.pack_name(data.ml_hostname)
        if data.ml_directory is None:
            raise TypeError('data.ml_directory == None')
        self.pack_dirpath(data.ml_directory)
        if data.ml_next is None:
            raise TypeError('data.ml_next == None')
        self.pack_mountlist(data.ml_next)

    def pack_groups(self, data):
        if len(data) > 1:
            raise XDRError('array length too long for data')
        self.pack_array(data, self.pack_groupnode)

    def pack_groupnode(self, data):
        if data.gr_name is None:
            raise TypeError('data.gr_name == None')
        self.pack_name(data.gr_name)
        if data.gr_next is None:
            raise TypeError('data.gr_next == None')
        self.pack_groups(data.gr_next)

    def pack_exports(self, data):
        if len(data) > 1:
            raise XDRError('array length too long for data')
        self.pack_array(data, self.pack_exportnode)

    def pack_exportnode(self, data):
        if data.ex_dir is None:
            raise TypeError('data.ex_dir == None')
        self.pack_dirpath(data.ex_dir)
        if data.ex_groups is None:
            raise TypeError('data.ex_groups == None')
        self.pack_groups(data.ex_groups)
        if data.ex_next is None:
            raise TypeError('data.ex_next == None')
        self.pack_exports(data.ex_next)


class nfs_pro_v3Unpacker(Unpacker):
    unpack_hyper = Unpacker.unpack_hyper
    unpack_string = Unpacker.unpack_string
    unpack_int = Unpacker.unpack_int
    unpack_float = Unpacker.unpack_float
    unpack_uint = Unpacker.unpack_uint
    unpack_opaque = Unpacker.unpack_opaque
    unpack_double = Unpacker.unpack_double
    unpack_unsigned = Unpacker.unpack_uint
    unpack_quadruple = Unpacker.unpack_double
    unpack_uhyper = Unpacker.unpack_uhyper
    unpack_bool = Unpacker.unpack_bool
    unpack_uint32 = unpack_uint

    def unpack_uint64(self):
        i = self._Unpacker__pos
        self._Unpacker__pos = j = i + 8
        data = self._Unpacker__buf[i:j]
        if len(data) < 8:
            raise EOFError
        return struct.unpack('>Q', data)[0]

    def unpack_filename3(self):
        data = self.unpack_string()
        return data

    def unpack_nfspath3(self):
        data = self.unpack_string()
        return data

    def unpack_cookieverf3(self):
        data = self.unpack_fopaque(const.NFS3_COOKIEVERFSIZE)
        return data

    def unpack_createverf3(self):
        data = self.unpack_fopaque(const.NFS3_CREATEVERFSIZE)
        return data

    def unpack_writeverf3(self):
        data = self.unpack_fopaque(const.NFS3_WRITEVERFSIZE)
        return data

    def unpack_nfsstat3(self):
        data = self.unpack_int()
        if data not in [const.NFS3_OK, const.NFS3ERR_PERM, const.NFS3ERR_NOENT, const.NFS3ERR_IO, const.NFS3ERR_NXIO,
                        const.NFS3ERR_ACCES, const.NFS3ERR_EXIST, const.NFS3ERR_XDEV, const.NFS3ERR_NODEV,
                        const.NFS3ERR_NOTDIR, const.NFS3ERR_ISDIR, const.NFS3ERR_INVAL, const.NFS3ERR_FBIG,
                        const.NFS3ERR_NOSPC, const.NFS3ERR_ROFS, const.NFS3ERR_MLINK, const.NFS3ERR_NAMETOOLONG,
                        const.NFS3ERR_NOTEMPTY, const.NFS3ERR_DQUOT, const.NFS3ERR_STALE, const.NFS3ERR_REMOTE,
                        const.NFS3ERR_BADHANDLE, const.NFS3ERR_NOT_SYNC, const.NFS3ERR_BAD_COOKIE,
                        const.NFS3ERR_NOTSUPP, const.NFS3ERR_TOOSMALL, const.NFS3ERR_SERVERFAULT, const.NFS3ERR_BADTYPE,
                        const.NFS3ERR_JUKEBOX]:
            raise XDRError('value=%s not in enum nfsstat3' % data)
        return data

    def unpack_ftype3(self):
        data = self.unpack_int()
        if data not in [const.NF3REG, const.NF3DIR, const.NF3BLK, const.NF3CHR, const.NF3LNK, const.NF3SOCK,
                        const.NF3FIFO]:
            raise XDRError('value=%s not in enum ftype3' % data)
        return data

    def unpack_specdata3(self, data_format='json'):
        data = types.specdata3()
        data.major = self.unpack_uint32()
        data.minor = self.unpack_uint32()
        return data.__dict__ if data_format == 'json' else data

    def unpack_nfs_fh3(self, data_format='json'):
        data = types.nfs_fh3()
        data.data = bytes(self.unpack_opaque())
        if len(data.data) > const.NFS3_FHSIZE:
            raise XDRError('array length too long for data.data')
        return data.__dict__ if data_format == 'json' else data

    def unpack_nfstime3(self, data_format='json'):
        data = types.nfstime3()
        data.seconds = self.unpack_uint32()
        data.nseconds = self.unpack_uint32()
        return data.__dict__ if data_format == 'json' else data

    def unpack_fattr3(self, data_format='json'):
        data = types.fattr3()
        data.type = self.unpack_ftype3()
        data.mode = self.unpack_uint32()
        data.nlink = self.unpack_uint32()
        data.uid = self.unpack_uint32()
        data.gid = self.unpack_uint32()
        data.size = self.unpack_uint64()
        data.used = self.unpack_uint64()
        data.rdev = self.unpack_specdata3()
        data.fsid = self.unpack_uint64()
        data.fileid = self.unpack_uint64()
        data.atime = self.unpack_nfstime3()
        data.mtime = self.unpack_nfstime3()
        data.ctime = self.unpack_nfstime3()
        return data.__dict__ if data_format == 'json' else data

    def unpack_post_op_attr(self, data_format='json'):
        data = types.post_op_attr()
        data.present = self.unpack_bool()
        if data.present == const.TRUE:
            data.attributes = self.unpack_fattr3(data_format)
        elif data.present == const.FALSE:
            pass
        else:
            raise XDRError('bad switch=%s' % data.present)
        return data.__dict__ if data_format == 'json' else data

    def unpack_wcc_attr(self, data_format='json'):
        data = types.wcc_attr()
        data.size = self.unpack_uint64()
        data.mtime = self.unpack_nfstime3(data_format)
        data.ctime = self.unpack_nfstime3(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_pre_op_attr(self, data_format='json'):
        data = types.pre_op_attr()
        data.present = self.unpack_bool()
        if data.present == const.TRUE:
            data.attributes = self.unpack_wcc_attr(data_format)
        elif data.present == const.FALSE:
            pass
        else:
            raise XDRError('bad switch=%s' % data.present)
        return data.__dict__ if data_format == 'json' else data

    def unpack_wcc_data(self, data_format='json'):
        data = types.wcc_data()
        data.before = self.unpack_pre_op_attr(data_format)
        data.after = self.unpack_post_op_attr(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_post_op_fh3(self, data_format='json'):
        data = types.post_op_fh3()
        data.present = self.unpack_bool()
        if data.present == const.TRUE:
            data.handle = self.unpack_nfs_fh3(data_format)
        elif data.present == const.FALSE:
            pass
        else:
            raise XDRError('bad switch=%s' % data.present)
        return data.__dict__ if data_format == 'json' else data

    def unpack_set_uint32(self, data_format='json'):
        data = types.set_uint32()
        data.set = self.unpack_bool()
        if data.set == const.TRUE:
            data.val = self.unpack_uint32()
        else:
            pass
        return data.__dict__ if data_format == 'json' else data

    def unpack_set_uint64(self, data_format='json'):
        data = types.set_uint64()
        data.set = self.unpack_bool()
        if data.set == const.TRUE:
            data.val = self.unpack_uint64()
        else:
            pass
        return data.__dict__ if data_format == 'json' else data

    def unpack_time_how(self):
        data = self.unpack_int()
        if data not in [const.DONT_CHANGE, const.SET_TO_SERVER_TIME, const.SET_TO_CLIENT_TIME]:
            raise XDRError('value=%s not in enum time_how' % data)
        return data

    def unpack_set_time(self, data_format='json'):
        data = types.set_time()
        data.set = self.unpack_time_how()
        if data.set == const.SET_TO_CLIENT_TIME:
            data.time = self.unpack_nfstime3(data_format)
        else:
            pass
        return data.__dict__ if data_format == 'json' else data

    def unpack_sattr3(self, data_format='json'):
        data = types.sattr3()
        data.mode = self.unpack_set_uint32()
        data.uid = self.unpack_set_uint32()
        data.gid = self.unpack_set_uint32()
        data.size = self.unpack_set_uint64()
        data.atime = self.unpack_set_time(data_format)
        data.mtime = self.unpack_set_time(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_diropargs3(self, data_format='json'):
        data = types.diropargs3()
        data.dir = self.unpack_nfs_fh3(data_format)
        data.name = self.unpack_filename3()
        return data.__dict__ if data_format == 'json' else data

    def unpack_diropres3ok(self, data_format='json'):
        data = types.diropres3ok()
        data.obj = self.unpack_post_op_fh3(data_format)
        data.obj_attributes = self.unpack_post_op_attr(data_format)
        data.dir_wcc = self.unpack_wcc_data(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_diropres3(self, data_format='json'):
        data = types.diropres3()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.resok = self.unpack_diropres3ok(data_format)
        else:
            data.resfail = self.unpack_wcc_data(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_wccstat3(self, data_format='json'):
        data = types.wccstat3()
        data.status = self.unpack_nfsstat3()
        if data.status == -1:
            pass
        else:
            data.wcc = self.unpack_wcc_data(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_getattr3res(self, data_format='json'):
        data = types.getattr3res()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.attributes = self.unpack_fattr3(data_format)
        else:
            pass
        return data.__dict__ if data_format == 'json' else data

    def unpack_sattrguard3(self, data_format='json'):
        data = types.sattrguard3()
        data.check = self.unpack_bool()
        if data.check == const.TRUE:
            data.ctime = self.unpack_nfstime3(data_format)
        elif data.check == const.FALSE:
            pass
        else:
            raise XDRError('bad switch=%s' % data.check)
        return data.__dict__ if data_format == 'json' else data

    def unpack_setattr3args(self, data_format='json'):
        data = types.setattr3args()
        data.object = self.unpack_nfs_fh3(data_format)
        data.new_attributes = self.unpack_sattr3(data_format)
        data.guard = self.unpack_sattrguard3(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_wccdata3res(self, category, data_format='json'):
        data = types.wcc_data3res(category=category)
        data.status = self.unpack_nfsstat3()
        data.wcc_data = self.unpack_wcc_data(data_format)
        if data_format == 'json':
            res = data.__dict__
            res.pop("category")
            if res["status"] == const.NFS3_OK:
                res["resok"] = res["wcc_data"]
            else:
                res["resfail"] = res["wcc_data"]
            res.pop("wcc_data")
            return res
        else:
            return data

    def unpack_setattr3res(self, data_format='json'):
        return self.unpack_wccdata3res(category='setattr3res', data_format=data_format)

    def unpack_lookup3resok(self, data_format='json'):
        data = types.lookup3resok()
        data.object = self.unpack_nfs_fh3(data_format)
        data.obj_attributes = self.unpack_post_op_attr(data_format)
        data.dir_attributes = self.unpack_post_op_attr(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_lookup3res(self, data_format='json'):
        data = types.lookup3res()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.resok = self.unpack_lookup3resok(data_format)
        else:
            data.resfail = self.unpack_post_op_attr(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_access3args(self, data_format='json'):
        data = types.access3args()
        data.object = self.unpack_nfs_fh3(data_format)
        data.access = self.unpack_uint32()
        return data.__dict__ if data_format == 'json' else data

    def unpack_access3resok(self, data_format='json'):
        data = types.access3resok()
        data.obj_attributes = self.unpack_post_op_attr(data_format)
        data.access = self.unpack_uint32()
        return data.__dict__ if data_format == 'json' else data

    def unpack_access3res(self, data_format='json'):
        data = types.access3res()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.resok = self.unpack_access3resok(data_format)
        else:
            data.resfail = self.unpack_post_op_attr(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_readlink3resok(self, data_format='json'):
        data = types.readlink3resok()
        data.symlink_attributes = self.unpack_post_op_attr(data_format)
        data.data = self.unpack_nfspath3()
        return data.__dict__ if data_format == 'json' else data

    def unpack_readlink3res(self, data_format='json'):
        data = types.readlink3res()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.resok = self.unpack_readlink3resok(data_format)
        else:
            data.resfail = self.unpack_post_op_attr(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_read3args(self, data_format='json'):
        data = types.read3args()
        data.file = self.unpack_nfs_fh3(data_format)
        data.offset = self.unpack_uint64()
        data.count = self.unpack_uint32()
        return data.__dict__ if data_format == 'json' else data

    def unpack_read3resok(self, data_format='json'):
        data = types.read3resok()
        data.file_attributes = self.unpack_post_op_attr(data_format)
        data.count = self.unpack_uint32()
        data.eof = self.unpack_bool()
        data.data = self.unpack_opaque()
        return data.__dict__ if data_format == 'json' else data

    def unpack_read3res(self, data_format='json'):
        data = types.read3res()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.resok = self.unpack_read3resok(data_format)
        else:
            data.resfail = self.unpack_post_op_attr(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_stable_how(self):
        data = self.unpack_int()
        if data not in [const.UNSTABLE, const.DATA_SYNC, const.FILE_SYNC]:
            raise XDRError('value=%s not in enum stable_how' % data)
        return data

    def unpack_write3args(self, data_format='json'):
        data = types.write3args()
        data.file = self.unpack_nfs_fh3(data_format)
        data.offset = self.unpack_uint64()
        data.count = self.unpack_uint32()
        data.stable = self.unpack_stable_how()
        data.data = self.unpack_opaque()
        return data.__dict__ if data_format == 'json' else data

    def unpack_write3resok(self, data_format='json'):
        data = types.write3resok()
        data.file_wcc = self.unpack_wcc_data(data_format)
        data.count = self.unpack_uint32()
        data.committed = self.unpack_stable_how()
        data.verf = self.unpack_writeverf3()
        return data.__dict__ if data_format == 'json' else data

    def unpack_write3res(self, data_format='json'):
        data = types.write3res()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.resok = self.unpack_write3resok(data_format)
        else:
            data.resfail = self.unpack_wcc_data(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_createmode3(self):
        data = self.unpack_int()
        if data not in [const.UNCHECKED, const.GUARDED, const.EXCLUSIVE]:
            raise XDRError('value=%s not in enum createmode3' % data)
        return data

    def unpack_createhow3(self, data_format='json'):
        data = types.createhow3()
        data.mode = self.unpack_createmode3()
        if data.mode == const.UNCHECKED or data.mode == const.GUARDED:
            data.obj_attributes = self.unpack_sattr3(data_format)
        elif data.mode == const.EXCLUSIVE:
            data.verf = self.unpack_createverf3()
        else:
            raise XDRError('bad switch=%s' % data.mode)
        return data.__dict__ if data_format == 'json' else data

    def unpack_create3args(self, data_format='json'):
        data = types.create3args()
        data.where = self.unpack_diropargs3(data_format)
        data.how = self.unpack_createhow3(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_create3resok(self, data_format='json'):
        data = types.create3resok()
        data.obj = self.unpack_post_op_fh3()
        data.obj_attributes = self.unpack_post_op_attr(data_format)
        data.dir_wcc = self.unpack_wcc_data(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_create3res(self, data_format='json'):
        data = types.create3res()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.resok = self.unpack_create3resok(data_format)
        else:
            data.resfail = self.unpack_wcc_data(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_mkdir3args(self, data_format='json'):
        data = types.mkdir3args()
        data.where = self.unpack_diropargs3(data_format)
        data.attributes = self.unpack_sattr3(data_format)
        return data.__dict__ if data_format == 'json' else data

    unpack_mkdir3res = unpack_create3res

    def unpack_symlinkdata3(self, data_format='json'):
        data = types.symlinkdata3()
        data.symlink_attributes = self.unpack_sattr3(data_format)
        data.symlink_data = self.unpack_nfspath3()
        return data.__dict__ if data_format == 'json' else data

    def unpack_symlink3args(self, data_format='json'):
        data = types.symlink3args()
        data.where = self.unpack_diropargs3(data_format)
        data.symlink = self.unpack_symlinkdata3(data_format)
        return data.__dict__ if data_format == 'json' else data

    unpack_symlink3res = unpack_create3res

    def unpack_devicedata3(self, data_format='json'):
        data = types.devicedata3()
        data.dev_attributes = self.unpack_sattr3(data_format)
        data.spec = self.unpack_specdata3(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_mknoddata3(self, data_format='json'):
        data = types.mknoddata3()
        data.type = self.unpack_ftype3()
        if data.type == const.NF3CHR or data.type == const.NF3BLK:
            data.device = self.unpack_devicedata3(data_format)
        elif data.type == const.NF3SOCK or data.type == const.NF3FIFO:
            data.pipe_attributes = self.unpack_sattr3(data_format)
        else:
            pass
        return data.__dict__ if data_format == 'json' else data

    def unpack_mknod3args(self, data_format='json'):
        data = types.mknod3args()
        data.where = self.unpack_diropargs3(data_format)
        data.what = self.unpack_mknoddata3(data_format)
        return data.__dict__ if data_format == 'json' else data

    unpack_mknod3res = unpack_create3res

    def unpack_remove3res(self, data_format='json'):
        return self.unpack_wccdata3res(category='remove3res', data_format=data_format)

    def unpack_rmdir3res(self, data_format='json'):
        return self.unpack_wccdata3res(category='rmdir3res', data_format=data_format)

    def unpack_rename3args(self, data_format='json'):
        data = types.rename3args()
        data.from_v = self.unpack_diropargs3(data_format)
        data.to = self.unpack_diropargs3(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_rename3wcc(self, data_format='json'):
        data = types.rename3wcc()
        data.fromdir_wcc = self.unpack_wcc_data(data_format)
        data.todir_wcc = self.unpack_wcc_data(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_rename3res(self, data_format='json'):
        data = types.rename3res()
        data.status = self.unpack_nfsstat3()
        if data.status == -1:
            pass
        else:
            data.res = self.unpack_rename3wcc(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_link3args(self, data_format='json'):
        data = types.link3args()
        data.file = self.unpack_nfs_fh3(data_format)
        data.link = self.unpack_diropargs3(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_link3wcc(self, data_format='json'):
        data = types.link3wcc()
        data.file_attributes = self.unpack_post_op_attr(data_format)
        data.linkdir_wcc = self.unpack_wcc_data(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_link3res(self, data_format='json'):
        data = types.link3res()
        data.status = self.unpack_nfsstat3()
        if data.status == -1:
            pass
        else:
            data.res = self.unpack_link3wcc(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_readdir3args(self, data_format='json'):
        data = types.readdir3args()
        data.dir = self.unpack_nfs_fh3(data_format)
        data.cookie = self.unpack_uint64()
        data.cookieverf = self.unpack_cookieverf3()
        data.count = self.unpack_uint32()
        return data.__dict__ if data_format == 'json' else data

    def unpack_entry3(self, data_format='json'):
        data = types.entry3()
        data.fileid = self.unpack_uint64()
        data.name = self.unpack_filename3()
        data.cookie = self.unpack_uint64()
        data.nextentry = self.unpack_array(self.unpack_entry3)
        if len(data.nextentry) > 1:
            raise XDRError('array length too long for data.nextentry')
        return data.__dict__ if data_format == 'json' else data

    def unpack_dirlist3(self, data_format='json'):
        data = types.dirlist3()
        data.entries = self.unpack_array(self.unpack_entry3)
        if len(data.entries) > 1:
            raise XDRError('array length too long for data.entries')
        data.eof = self.unpack_bool()
        return data.__dict__ if data_format == 'json' else data

    def unpack_readdir3resok(self, data_format='json'):
        data = types.readdir3resok()
        data.dir_attributes = self.unpack_post_op_attr(data_format)
        data.cookieverf = self.unpack_cookieverf3()
        data.reply = self.unpack_dirlist3(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_readdir3res(self, data_format='json'):
        data = types.readdir3res()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.resok = self.unpack_readdir3resok(data_format)
        else:
            data.resfail = self.unpack_post_op_attr(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_readdirplus3args(self, data_format='json'):
        data = types.readdirplus3args()
        data.dir = self.unpack_nfs_fh3(data_format)
        data.cookie = self.unpack_uint64()
        data.cookieverf = self.unpack_cookieverf3()
        data.dircount = self.unpack_uint32()
        data.maxcount = self.unpack_uint32()
        return data.__dict__ if data_format == 'json' else data

    def unpack_entryplus3(self, data_format='json'):
        data = types.entryplus3()
        data.fileid = self.unpack_uint64()
        data.name = self.unpack_filename3()
        data.cookie = self.unpack_uint64()
        data.name_attributes = self.unpack_post_op_attr(data_format)
        data.name_handle = self.unpack_post_op_fh3(data_format)
        data.nextentry = self.unpack_array(self.unpack_entryplus3)
        if len(data.nextentry) > 1:
            raise XDRError('array length too long for data.nextentry')
        return data.__dict__ if data_format == 'json' else data

    def unpack_dirlistplus3(self, data_format='json'):
        data = types.dirlistplus3()
        data.entries = self.unpack_array(self.unpack_entryplus3)
        if len(data.entries) > 1:
            raise XDRError('array length too long for data.entries')
        data.eof = self.unpack_bool()
        return data.__dict__ if data_format == 'json' else data

    def unpack_readdirplus3resok(self, data_format='json'):
        data = types.readdirplus3resok()
        data.dir_attributes = self.unpack_post_op_attr(data_format)
        data.cookieverf = self.unpack_cookieverf3()
        data.reply = self.unpack_dirlistplus3(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_readdirplus3res(self, data_format='json'):
        data = types.readdirplus3res()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.resok = self.unpack_readdirplus3resok(data_format)
        else:
            data.resfail = self.unpack_post_op_attr(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_fsstat3resok(self, data_format='json'):
        data = types.fsstat3resok()
        data.obj_attributes = self.unpack_post_op_attr(data_format)
        data.tbytes = self.unpack_uint64()
        data.fbytes = self.unpack_uint64()
        data.abytes = self.unpack_uint64()
        data.tfiles = self.unpack_uint64()
        data.ffiles = self.unpack_uint64()
        data.afiles = self.unpack_uint64()
        data.invarsec = self.unpack_uint32()
        return data.__dict__ if data_format == 'json' else data

    def unpack_fsstat3res(self, data_format='json'):
        data = types.fsstat3res()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.resok = self.unpack_fsstat3resok(data_format)
        else:
            data.resfail = self.unpack_post_op_attr(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_fsinfo3resok(self, data_format='json'):
        data = types.fsinfo3resok()
        data.obj_attributes = self.unpack_post_op_attr(data_format)
        data.rtmax = self.unpack_uint32()
        data.rtpref = self.unpack_uint32()
        data.rtmult = self.unpack_uint32()
        data.wtmax = self.unpack_uint32()
        data.wtpref = self.unpack_uint32()
        data.wtmult = self.unpack_uint32()
        data.dtpref = self.unpack_uint32()
        data.maxfilesize = self.unpack_uint64()
        data.time_delta = self.unpack_nfstime3(data_format)
        data.properties = self.unpack_uint32()
        return data.__dict__ if data_format == 'json' else data

    def unpack_fsinfo3res(self, data_format='json'):
        data = types.fsinfo3res()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.resok = self.unpack_fsinfo3resok(data_format)
        else:
            data.resfail = self.unpack_post_op_attr(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_pathconf3resok(self, data_format='json'):
        data = types.pathconf3resok()
        data.obj_attributes = self.unpack_post_op_attr(data_format)
        data.linkmax = self.unpack_uint32()
        data.name_max = self.unpack_uint32()
        data.no_trunc = self.unpack_bool()
        data.chown_restricted = self.unpack_bool()
        data.case_insensitive = self.unpack_bool()
        data.case_preserving = self.unpack_bool()
        return data.__dict__ if data_format == 'json' else data

    def unpack_pathconf3res(self, data_format='json'):
        data = types.pathconf3res()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.resok = self.unpack_pathconf3resok(data_format)
        else:
            data.resfail = self.unpack_post_op_attr(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_commit3args(self, data_format='json'):
        data = types.commit3args()
        data.file = self.unpack_nfs_fh3(data_format)
        data.offset = self.unpack_uint64()
        data.count = self.unpack_uint32()
        return data.__dict__ if data_format == 'json' else data

    def unpack_commit3resok(self, data_format='json'):
        data = types.commit3resok()
        data.file_wcc = self.unpack_wcc_data(data_format)
        data.verf = self.unpack_writeverf3()
        return data.__dict__ if data_format == 'json' else data

    def unpack_commit3res(self, data_format='json'):
        data = types.commit3res()
        data.status = self.unpack_nfsstat3()
        if data.status == const.NFS3_OK:
            data.resok = self.unpack_commit3resok(data_format)
        else:
            data.resfail = self.unpack_wcc_data(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_setaclargs(self, data_format='json'):
        data = types.setaclargs()
        data.dargs = self.unpack_diropargs3(data_format)
        data.wargs = self.unpack_write3args(data_format)
        return data.__dict__ if data_format == 'json' else data

    def unpack_dirpath(self):
        data = self.unpack_string()
        if len(data) > const.NFS3_MNTPATHLEN:
            raise XDRError('array length too long for data')
        return data

    def unpack_name(self):
        data = self.unpack_string()
        if len(data) > const.NFS3_MNTNAMLEN:
            raise XDRError('array length too long for data')
        return data

    def unpack_fhandle3(self):
        data = self.unpack_opaque()
        if len(data) > const.NFS3_FHSIZE:
            raise XDRError('array length too long for data')
        return data

    def unpack_mountstat3(self):
        data = self.unpack_int()
        if data not in [const.MNT3_OK, const.MNT3ERR_PERM, const.MNT3ERR_NOENT, const.MNT3ERR_IO, const.MNT3ERR_ACCES,
                        const.MNT3ERR_NOTDIR, const.MNT3ERR_INVAL, const.MNT3ERR_NAMETOOLONG, const.MNT3ERR_NOTSUPP,
                        const.MNT3ERR_SERVERFAULT]:
            raise XDRError('value=%s not in enum mountstat3' % data)
        return data

    def unpack_mountres3_ok(self, data_format='json'):
        data = types.mountres3_ok()
        data.fhandle = self.unpack_fhandle3()
        data.auth_flavors = self.unpack_array(self.unpack_int)
        return data.__dict__ if data_format == 'json' else data

    def unpack_mountres3(self, data_format='json'):
        data = types.mountres3()
        data.status = self.unpack_mountstat3()
        if data.status == const.MNT3_OK:
            data.mountinfo = self.unpack_mountres3_ok(data_format)
        else:
            pass
        return data.__dict__ if data_format == 'json' else data

    def unpack_mountlist(self):
        data = self.unpack_array(self.unpack_mountbody)
        if len(data) > 1:
            raise XDRError('array length too long for data')
        return data

    def unpack_mountbody(self, data_format='json'):
        data = types.mountbody()
        data.ml_hostname = self.unpack_name()
        data.ml_directory = self.unpack_dirpath()
        data.ml_next = self.unpack_mountlist()
        return data.__dict__ if data_format == 'json' else data

    def unpack_groups(self):
        data = self.unpack_array(self.unpack_groupnode)
        if len(data) > 1:
            raise XDRError('array length too long for data')
        return data

    def unpack_groupnode(self):
        data = types.groupnode()
        data.gr_name = self.unpack_name()
        data.gr_next = self.unpack_groups()
        return data

    def unpack_exports(self):
        data = self.unpack_array(self.unpack_exportnode)
        if len(data) > 1:
            raise XDRError('array length too long for data')
        return data

    def unpack_exportnode(self):
        data = types.exportnode()
        data.ex_dir = self.unpack_dirpath()
        data.ex_groups = self.unpack_groups()
        data.ex_next = self.unpack_exports()
        return data
