#!/usr/bin/python
# -*- coding: UTF-8 -*-

import random

class Msg:
    def __init__(self, sender, rid, mid, dh):
        self.sender = sender
        self.rid = rid
        self.mid = mid
        self.dh = dh

class Entity:
    'A participant'

    def __init__(self, name):
        self.name = name
        self.role = 'A' if (name == 'Alice') else 'B'
        self.our_dh = [None,None]
        self.their_dh = None
        self.R = []
        self.Ca = []
        self.Cb = []
        self.rid = 0
        self.mid = 0
        self.r_flag = False

    def display(self):
        print self.__dict__

    def send(self):
        if self.r_flag == True:
            print self.name, '\tRatcheting...'
            self.genDH()
            self.rid+=1
            self.mid=0
            self.derive()
            print self.name, '\tnext our_dh:', self.our_dh, 'key:', self.our_dh[1]+self.their_dh
            self.r_flag = False
        if self.role == 'A':
            self.Ca[self.rid]['chain'].append(self.mid)
        else:
            self.Cb[self.rid]['chain'].append(self.mid)
        toSend = Msg(self.name, self.rid, self.mid, self.our_dh[1])
        self.mid+=1
        print self.name, '\tsending: ', toSend.__dict__
        return toSend

    def receive(self, m):
        print self.name, "\treceive: ", m.__dict__
        if m.rid >= self.rid:
            self.their_dh = m.dh
            self.r_flag = True
        print self.name, "\tkey: ", self.our_dh[1]+self.their_dh

    def genDH(self):
        self.our_dh = [random.randint(0,1024), random.randint(0,1024)]

    def derive(self):
        i = self.rid
        self.R.append(i)
        self.Ca.append({'rid':i,'chain':[]})
        self.Cb.append({'rid':i,'chain':[]})

def initialize():
    print "Initializing"
    a = Entity('Alice')
    b = Entity('Bob')

    b.our_dh = [random.randint(0,1024), random.randint(0,1024)]
    a.our_dh = [random.randint(0,1024), random.randint(0,1024)]

    b.derive()
    b.r_flag = False

    a.derive()
    a.r_flag = True


    a.their_dh = b.our_dh[1]
    b.their_dh = a.our_dh[1]

    return a, b

a, b = initialize()
a.receive(b.send())
a.receive(b.send())
a.receive(b.send())
a.receive(b.send())
b.receive(a.send())
b.receive(a.send())
b.receive(a.send())
a.receive(b.send())
a.receive(b.send())
a.receive(b.send())
a.receive(b.send())
b.receive(a.send())
b.receive(a.send())
b.receive(a.send())
