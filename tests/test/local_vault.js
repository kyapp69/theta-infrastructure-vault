const chai = require('chai')
const expect = chai.expect
chai.use(require('chai-as-promised'))

const Wallet = require('../client');

const default_config = {
    reserve_batch_size: 4,
    submit_payment_timer: 6 * 1000
};

var getRandomInt = (max) => {
    return Math.floor(Math.random() * Math.floor(max));
}
var testID = getRandomInt(10000000);
var getRandomUserID = () => {
    return `local_vault_test_${testID}_user_${getRandomInt(1000000000)}`;
}
var getRandomUserWallet = () => {
    let userID = getRandomUserID()
    return new Wallet(Object.assign({}, default_config, {
        xuser: userID
    }));
}
var sleep = (ms) => {
    return new Promise(resolve => setTimeout(resolve, ms));
}

describe('Local vault', () => {
    describe('sanity', () => {
        it('should be able to make RPC request', async () => {
            let w1 = getRandomUserWallet();

            let acc1 = await w1.getAccount();

            expect(acc1.user_id).not.to.be.undefined;
        });
    });

    describe('APIs', () => {
        describe('_getAccount', () => {
            it('should be able to query a new account', async () => {
                let w1 = getRandomUserWallet();
                let acc1 = await w1._getAccount()

                expect(acc1.user_id).not.to.be.undefined;
                expect(acc1.send_account).not.to.be.undefined;
                expect(acc1.recv_account).not.to.be.undefined;

                expect(acc1.send_account.coins).to.be.undefined;
                expect(acc1.recv_account.coins).to.be.undefined;

                // wait for facuet to inject funds.
                await sleep(5000);

                acc1 = await w1._getAccount();
                expect(acc1.user_id).not.to.be.undefined;
                expect(acc1.send_account).not.to.be.undefined;
                expect(acc1.recv_account).not.to.be.undefined;
                expect(acc1.send_account.coins).not.to.be.undefined;
                expect(acc1.send_account.coins.gammawei).to.be.above(0);

                expect(acc1.recv_account.coins).to.be.undefined;
            });
        });

        describe('_reserveFund/_createPayment/_submitPayment', () => {
            it('should pay correctly', async () => {
                let alice = getRandomUserWallet();
                let bob = getRandomUserWallet();

                await alice.getAccount();
                await bob.getAccount();

                // wait for facuet to inject funds.
                await sleep(5000);

                let aliceAcc = aliceAcc1 = await alice.getAccount();
                let bobAcc = bobAcc1 = await bob.getAccount();

                expect(aliceAcc.send_account.coins).not.to.be.undefined;
                expect(aliceAcc.send_account.coins.gammawei).to.be.above(5000)

                let resourceID = 'die_another_day';
                let reserveResp = await alice._reserveFund(resourceID, 1000, 1001);
                expect(reserveResp).not.to.be.undefined;
                expect(reserveResp.reserve_sequence).to.be.above(0)

                let reserveSeq = reserveResp.reserve_sequence;
                aliceAcc = await alice.getAccount()
                let paymentResp = await alice._createPayment(bobAcc.recv_account.address,
                    500, reserveSeq, aliceAcc.send_account.sequence + 1, resourceID);
                expect(paymentResp).not.to.be.undefined;
                expect(paymentResp.payment).not.to.be.undefined;

                let payment = paymentResp.payment;
                let submitResp = await bob._submitPayment(payment);
                expect(submitResp).not.to.be.undefined;

                // wait for payment to be included in blockchain.
                await sleep(5000);

                let aliceAcc2 = await alice.getAccount();
                let bobAcc2 = await bob.getAccount();

                expect(aliceAcc2.send_account.coins.gammawei).to.equal(
                    aliceAcc1.send_account.coins.gammawei - 1000 - 1001 - 1/* tx fee for reserve fund */);
                expect(bobAcc2.recv_account.coins).not.to.be.undefined;
                expect(bobAcc2.recv_account.coins.gammawei).to.equal(500 - 1/* tx fee for submit payment */);
            });
        });
    });
});