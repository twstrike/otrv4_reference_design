package main

import (
	"bytes"
	"fmt"

	"golang.org/x/crypto/sha3"

	"github.com/twstrike/ed448"
)

const (
	Q = iota
	P1
	P2
	D
)

type Msg struct {
	mtype    int
	sender   string
	rid, mid int
	dh       pubkey

	encKey key // this is here only to check if we can decrypt
}

func (m Msg) decryptWith(k key) {
	if !bytes.Equal(k, m.encKey) {
		panic("failed to decrypt message.")
	}
}

var c = ed448.NewCurve()
var NULLSEC = seckey{}
var NULLPUB = pubkey{}

type seckey [144]byte
type pubkey [56]byte
type key []byte

type Entity struct {
	name                 string
	our_dh_pub, their_dh pubkey
	our_dh_priv          seckey
	R                    []key
	Ca, Cb               []key
	rid, j, k            int
}

func (e *Entity) sendData() Msg {
	var cj key
	if e.j == 0 {
		fmt.Println()
		fmt.Printf("%s \tRatcheting...\n", e.name)
		e.our_dh_priv, e.our_dh_pub, _ = c.GenerateKeys()
		e.rid += 1
		secret := c.ComputeSecret(e.our_dh_priv, e.their_dh)
		e.derive(secret[:])
	}

	cj = e.retriveChainkey(e.rid, e.j)
	toSend := Msg{D, e.name, e.rid, e.j, e.our_dh_pub, cj}
	e.j += 1

	fmt.Printf("%s \tsending: %v\n", e.name, toSend)
	fmt.Printf("%s \tour key: %x\n", e.name, cj)
	return toSend
}

func (e *Entity) receive(m Msg) {
	fmt.Println()
	fmt.Printf("%s \treceive: %v\n", e.name, m)
	switch m.mtype {
	case D:
		e.receiveData(m)
		break
	case Q:
		break
	case P1:
		e.receiveP1(m)
		break
	case P2:
		e.receiveP2(m)
		break
	}
}

func (e *Entity) receiveP1(m Msg) {
	e.their_dh = m.dh
	e.rid = e.rid + 1
	if bytes.Compare(e.our_dh_priv[:], NULLSEC[:]) == 1 {
		secret := c.ComputeSecret(e.our_dh_priv, e.their_dh)
		e.derive(secret[:])
	}
}

func (e *Entity) receiveP2(m Msg) {
	e.their_dh = m.dh
	e.rid = e.rid + 1

	secret := c.ComputeSecret(e.our_dh_priv, e.their_dh)
	e.derive(secret[:])
}

func (e *Entity) receiveData(m Msg) {
	ck := make([]byte, 64)
	if m.rid == e.rid+1 {
		fmt.Printf("%s \tFollow Ratcheting...\n", e.name)
		e.rid = m.rid
		e.their_dh = m.dh
		secret := c.ComputeSecret(e.our_dh_priv, e.their_dh)
		e.derive(secret[:])
		e.j = 0 // need to ratchet next time when send
	} else if e.k > m.mid {
		//panic("we received a message delayed out of order")
	}

	e.k = m.mid
	ck = e.retriveChainkey(m.rid, m.mid)
	fmt.Printf("%s \ttheir key: %x\n", e.name, ck)

	m.decryptWith(ck)
}

func (e *Entity) wasAliceAt(rid int) bool {
	return rid%2 == 1
}

func (e *Entity) retriveChainkey(rid, mid int) key {
	var ck key
	buf := make([]byte, 64)

	if e.wasAliceAt(rid) {
		ck = e.Ca[rid]
	} else {
		ck = e.Cb[rid]
	}
	copy(buf, ck)
	for i := mid; i > 0; i-- {
		sha3.ShakeSum256(buf, buf)
	}
	return buf
}

func (e *Entity) derive(secret []byte) {
	r := make([]byte, 64)
	ca := make([]byte, 64)
	cb := make([]byte, 64)
	if len(e.R) > 0 {
		secret = append(secret, e.R[e.rid-1]...)
	}
	sha3.ShakeSum256(r, append(secret, 0))
	sha3.ShakeSum256(ca, append(secret, 1))
	sha3.ShakeSum256(cb, append(secret, 2))

	e.R = append(e.R, r)
	e.Ca = append(e.Ca, ca)
	e.Cb = append(e.Cb, cb)
}

func (e *Entity) query() Msg {
	toSend := Msg{mtype: Q, sender: e.name}
	fmt.Printf("%s \tsending: %v\n", e.name, toSend)
	return toSend
}

func (e *Entity) sendP1() Msg {
	e.our_dh_priv, e.our_dh_pub, _ = c.GenerateKeys()

	e.j = 1
	e.rid = e.rid + 1
	if bytes.Compare(e.their_dh[:], NULLPUB[:]) == 1 {
		secret := c.ComputeSecret(e.our_dh_priv, e.their_dh)
		e.derive(secret[:])
	}

	toSend := Msg{P1, e.name, -1, -1, e.our_dh_pub, nil}
	fmt.Printf("%s \tsending: %v\n", e.name, toSend)
	return toSend
}

func (e *Entity) sendP2() Msg {
	e.j = 0
	e.rid = e.rid + 1
	e.our_dh_priv, e.our_dh_pub, _ = c.GenerateKeys()
	secret := c.ComputeSecret(e.our_dh_priv, e.their_dh)
	e.derive(secret[:])

	toSend := Msg{P2, e.name, -1, -1, e.our_dh_pub, nil}
	fmt.Printf("%s \tsending: %v\n", e.name, toSend)
	return toSend
}

func main() {
	var a, b *Entity

	//XXX They are all good so far
	if true {
		fmt.Println("=========================")
		fmt.Println("Testing fresh DAKE")
		fmt.Println("=========================")

		runFreshDAKE()

		fmt.Println("=========================")
		fmt.Println("Testing sync data message")
		fmt.Println("=========================")

		testSyncDataMessages(runFreshDAKE())

		fmt.Println("=========================")
		fmt.Println("Testing async data message")
		fmt.Println("=========================")

		testAsyncDataMessages(runFreshDAKE())

		fmt.Println("=========================")
		fmt.Println("Testing new sync DAKE")
		fmt.Println("=========================")

		// a sends first, will start a new ratchet
		testSyncDataMessages(runFreshDAKE())

		a, b = runFreshDAKE()
		// b sends first, meaning it should start by sending a follow up msg
		testSyncDataMessages(b, a)

		fmt.Println("=========================")
		fmt.Println("Testing async DAKE message - Late msg is a follow up")
		fmt.Println("=========================")

		a, b = runFreshDAKE()
		testSyncDataMessages(a, b)

		//B will send a late msg during a new DAKE.
		a.receive(b.sendData()) //Make sure late msg is a follow up
		testAsyncDAKE_AliceReceivesLateMsgFromPreviousDAKE(a, b)
		testSyncDataMessages(a, b) // Alice should ratchet because she sends first

		a, b = runFreshDAKE()
		testSyncDataMessages(a, b)

		//B will send a late msg during a new DAKE.
		a.receive(b.sendData()) //Make sure late msg is a follow up
		testAsyncDAKE_AliceReceivesLateMsgFromPreviousDAKE(a, b)
		testSyncDataMessages(b, a) // Bob should not ratchet because he sends first

		fmt.Println("=========================")
		fmt.Println("Testing async DAKE message - Late msg is a new RATCHET")
		fmt.Println("=========================")

		a, b = runFreshDAKE()
		testSyncDataMessages(a, b)

		//B will send a late msg during a new DAKE.
		b.receive(a.sendData()) //Make sure late msg is a new RATCHET
		testAsyncDAKE_AliceReceivesLateMsgFromPreviousDAKE(a, b)
		testSyncDataMessages(a, b) // Alice should ratchet because she sends first

		a, b = runFreshDAKE()
		testSyncDataMessages(a, b)

		//B will send a late msg during a new DAKE.
		b.receive(a.sendData()) //Make sure late msg is a new RATCHET
		testAsyncDAKE_AliceReceivesLateMsgFromPreviousDAKE(a, b)
		testSyncDataMessages(b, a) // Bob should not ratchet because he sends first
	}

	//TODO FIXME
	if false {
		fmt.Println("=========================")
		fmt.Println("Testing async DAKE message - Alice receive late after she ratchet")
		fmt.Println("=========================")

		a, b = runFreshDAKE()
		testSyncDataMessages(a, b)

		//B will send a late msg during a new DAKE.
		a.receive(b.sendData()) //Make sure late msg is a follow up
		testAsyncDAKE_AliceReceivesLateMsgFromPreviousDAKEAfterSheRatchetsAgain(a, b)
		testSyncDataMessages(a, b) // Alice should ratchet because she sends first

		a, b = runFreshDAKE()
		testSyncDataMessages(a, b)

		//B will send a late msg during a new DAKE.
		a.receive(b.sendData()) //Make sure late msg is a new RATCHET
		testAsyncDAKE_AliceReceivesLateMsgFromPreviousDAKEAfterSheRatchetsAgain(a, b)
		testSyncDataMessages(a, b) // Alice should ratchet because she sends first

		fmt.Println("=========================")
		fmt.Println("Testing async DAKE message - RATCHET over DAKE")
		fmt.Println("=========================")

		a, b = runFreshDAKE()
		testSyncDataMessages(a, b)

		//B will send a late msg during a new DAKE.
		a.receive(b.sendData()) //Make sure late msg is a follow up
		testAsyncDAKE_AliceReceivesLateNewRathcetMsgFromPreviousDAKE(a, b)
		testSyncDataMessages(a, b) // Alice should ratchet because she sends first

		a, b = runFreshDAKE()
		testSyncDataMessages(a, b)

		//B will send a late msg during a new DAKE.
		a.receive(b.sendData()) //Make sure late msg is a new RATCHET
		testAsyncDAKE_AliceReceivesLateNewRathcetMsgFromPreviousDAKE(a, b)
		testSyncDataMessages(a, b) // Alice should ratchet because she sends first
	}

	//
	// OLD TEST
	//

	a, b = initialize()
	b.receive(a.query())
	a.receive(b.sendP1())
	b.receive(a.sendP2())

	fmt.Println("=========================")
	fmt.Println("Testing sync data message")
	fmt.Println("=========================")

	a.receive(b.sendData()) // b sends first, so no new ratchet happens.
	a.receive(b.sendData()) // b again: this is another follow up msg.
	b.receive(a.sendData()) // a sends, a new ratchet happens and bob follows.
	b.receive(a.sendData()) // a again: this is a follow up.

	fmt.Println("=========================")
	fmt.Println("Testing async data message")
	fmt.Println("=========================")

	m1 := a.sendData() // a sends again: another follow up message.
	m2 := b.sendData() // b sends now, a new ratcher happens for bob.
	m3 := a.sendData() // a sends again: another follow up message.

	b.receive(m1) // b receives follow up message from a previous ratchet.
	b.receive(m3) // b receives follow up message from a previous ratchet.
	a.receive(m2) // a receives a message from a new ratchet. She follows the ratchet.

	fmt.Println("=========================")
	fmt.Println("Testing new sync DAKE")
	fmt.Println("=========================")

	b.receive(a.query())
	a.receive(b.sendP1())
	b.receive(a.sendP2())

	b.receive(a.sendData()) // a sends, a new ratchet starts and bob follows
	b.receive(a.sendData()) // a sends a follow up
	a.receive(b.sendData()) // b sends, a new ratchet starts and alice follows
	a.receive(b.sendData()) // b sends a follow up

	fmt.Println("=========================")
	fmt.Println("Testing async DAKE message")
	fmt.Println("=========================")

	a.receive(b.sendData()) // make sure b0 is a follow up

	b.receive(a.query())
	p1 := b.sendP1()
	b0 := b.sendData() // bob sends a data message during a new DAKE, is this a follow up msg?
	b1 := b.sendData() // bob sends a data message during a new DAKE - surely a follow up msg.

	//FIXME
	b.receive(a.sendData()) // a sends a new message before she receives p1, but after bob sends p1.
	// this will be a new ratchet, and thats a problem because bob will also ratchet when sending p1.

	a.receive(p1)      // a receives p1
	p2 := a.sendP2()   // ... and immediately replies with a p2
	a0 := a.sendData() // ... and send a new data msg

	b.receive(p2) // bob receives a p2
	b.receive(a0) // and the a0

	a.receive(b0) // a receives b0 (I want to see how it works if she receives this BEFORE sending a0)
	a.receive(b1) // a receives b1

	// After delayed messages, happy path
	a.receive(b.sendData()) // b sends, a new ratchet starts and alice follows
	a.receive(b.sendData()) // b sends a follow up
	b.receive(a.sendData()) // a sends, a new ratchet starts and bob follows
	b.receive(a.sendData()) // a sends a follow up
}

func initialize() (alice, bob *Entity) {
	alice = new(Entity)
	bob = new(Entity)

	alice.name = "Alice"
	alice.rid = -2
	bob.name = "Bob"
	bob.rid = -2

	return
}

func runFreshDAKE() (a, b *Entity) {
	return testSyncDAKE(initialize())
}

func testSyncDAKE(a, b *Entity) (*Entity, *Entity) {
	b.receive(a.query())
	a.receive(b.sendP1())
	b.receive(a.sendP2())

	return a, b
}

func testSyncDataMessages(a, b *Entity) {
	a.receive(b.sendData()) // b sends first, so no new ratchet happens.
	a.receive(b.sendData()) // b again: this is another follow up msg.
	b.receive(a.sendData()) // a sends, a new ratchet happens and bob follows.
	b.receive(a.sendData()) // a again: this is a follow up.
}

func testAsyncDataMessages(a, b *Entity) {
	b.receive(a.sendData()) // enforce m1 is a follow up
	m1 := a.sendData()      // a sends again: another follow up message.
	m2 := b.sendData()      // b sends now, a new ratcher happens for bob.
	m3 := a.sendData()      // a sends again: another follow up message.

	b.receive(m1) // b receives follow up message from a previous ratchet.
	b.receive(m3) // b receives follow up message from a previous ratchet.
	a.receive(m2) // a receives a message from a new ratchet. She follows the ratchet.
}

// NOTE The late message may or may not be a follow up.
// NOTE Bob does not receive any message after starting the DAKE.
// NOTE Bob does not receive any late messages after both finish the DAKE.
func testAsyncDAKE_AliceReceivesLateMsgFromPreviousDAKE(a, b *Entity) {
	b.receive(a.query())
	p1 := b.sendP1()

	// Bob sends a message which wiill be delivered late. It can be a follow up or not.
	late := b.sendData()

	// NOTE Bob does not receive any message after starting the DAKE.

	a.receive(p1)    // a receives p1
	p2 := a.sendP2() // ... and immediately replies with a p2. The DAKE finishes for Alice.

	// Alice receives the late message after finishing the DAKE
	a.receive(late)

	// AKE finishes for Bob.
	b.receive(p2)

	// NOTE Bob does not receive any late messages from Alice.
	// This can only happen if she do not receive P1.
}

// NOTE The late message may or may not be a follow up.
// NOTE Bob does not receive any message after starting the DAKE.
// NOTE Bob does not receive any late messages after both finish the DAKE.
// NOTE Alice will start a NEW ratchet before reeives the late message.
func testAsyncDAKE_AliceReceivesLateMsgFromPreviousDAKEAfterSheRatchetsAgain(a, b *Entity) {
	b.receive(a.query())
	p1 := b.sendP1()

	// Bob sends a message which wiill be delivered late. It can be a follow up or not.
	late := b.sendData()

	b.receive(a.sendData()) // Alice starts a NEW ratchet.

	// NOTE Bob does not receive any message after starting the DAKE.

	a.receive(p1)    // a receives p1
	p2 := a.sendP2() // ... and immediately replies with a p2. The DAKE finishes for Alice.

	late_from_receiver := a.sendData() // This should make Alice ratchet

	// Alice receives the late message after finishing the DAKE
	a.receive(late)

	// AKE finishes for Bob.
	b.receive(p2)
	b.receive(late_from_receiver)

	// NOTE Bob does not receive any late messages from Alice.
	// This can only happen if she do not receive P1.
}

func testAsyncDAKE_AliceReceivesLateNewRathcetMsgFromPreviousDAKE(a, b *Entity) {
	b.receive(a.query())
	p1 := b.sendP1()

	late := b.sendData()    // Bob sends late. Can be NEW ratchet or follow up.
	b.receive(a.sendData()) // Bob receives from Alice. If "late" is a follow up, this is a NEW ratchet. This is a follow up otherwise.
	late2 := b.sendData()   // Bob sends late2. This is always a NEW dake (he has just receive something from Alice).
	b.receive(a.sendData()) // Alice sends a follow up (she hasnt received anything from Bob), since her last message.

	a.receive(p1)    // a receives p1
	p2 := a.sendP2() // ... and immediately replies with a p2. The DAKE finishes for Alice.

	// AKE finishes for Bob.
	b.receive(p2)

	// Alice receives the late message after finishing the DAKE
	a.receive(late)
	a.receive(late2)
}
