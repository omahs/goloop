package example;

import avm.Address;
import avm.Blockchain;
import foundation.icon.ee.tooling.abi.External;
import foundation.icon.ee.tooling.abi.EventLog;

import java.math.BigInteger;

public class APITest
{
    public static void onInstall() {
    }

    @EventLog
    public static void EmitEvent(byte[] data) {}

    //================================
    // Address
    //================================

    @External
    public static void getAddress(Address addr) {
        Blockchain.require(Blockchain.getAddress().equals(addr));
    }

    @External(readonly=true)
    public static Address getAddressQuery() {
        return Blockchain.getAddress();
    }

    @External
    public static void getCaller(Address caller) {
        Blockchain.require(Blockchain.getCaller().equals(caller));
    }

    @External(readonly=true)
    public static Address getCallerQuery() {
        return Blockchain.getCaller();
    }

    @External
    public static void getOrigin(Address origin) {
        Blockchain.require(Blockchain.getOrigin().equals(origin));
    }

    @External(readonly=true)
    public static Address getOriginQuery() {
        return Blockchain.getOrigin();
    }

    @External
    public static void getOwner(Address owner) {
        Blockchain.require(Blockchain.getOwner().equals(owner));
    }

    @External(readonly=true)
    public static Address getOwnerQuery() {
        return Blockchain.getOwner();
    }

    @External
    public static BigInteger getValue() {
        return Blockchain.getValue();
    }

    @External(readonly=true)
    public static BigInteger getValueQuery() {
        return Blockchain.getValue();
    }

    //================================
    // Block
    //================================

    @External
    public static void getBlockTimestamp() {
        Blockchain.require(Blockchain.getBlockTimestamp() > 0L);
        EmitEvent(BigInteger.valueOf(Blockchain.getBlockTimestamp()).toByteArray());
    }

    @External(readonly=true)
    public static long getBlockTimestampQuery() {
        return Blockchain.getBlockTimestamp();
    }

    @External
    public static void getBlockHeight() {
        Blockchain.require(Blockchain.getBlockHeight() > 0L);
        EmitEvent(BigInteger.valueOf(Blockchain.getBlockHeight()).toByteArray());
    }

    @External(readonly=true)
    public static long getBlockHeightQuery() {
        return Blockchain.getBlockHeight();
    }

    //================================
    // Transaction
    //================================

    @External
    public static void getTransactionHash() {
        Blockchain.require(Blockchain.getTransactionHash() != null);
        EmitEvent(Blockchain.getTransactionHash());
    }

    @External(readonly=true)
    public static byte[] getTransactionHashQuery() {
        return Blockchain.getTransactionHash();
    }

    @External
    public static void getTransactionIndex() {
        Blockchain.require(Blockchain.getTransactionIndex() >= 0);
        EmitEvent(BigInteger.valueOf(Blockchain.getTransactionIndex()).toByteArray());
    }

    @External(readonly=true)
    public static int getTransactionIndexQuery() {
        return Blockchain.getTransactionIndex();
    }

    @External
    public static void getTransactionTimestamp() {
        Blockchain.require(Blockchain.getTransactionTimestamp() > 0L);
        EmitEvent(BigInteger.valueOf(Blockchain.getTransactionTimestamp()).toByteArray());
    }

    @External(readonly=true)
    public static long getTransactionTimestampQuery() {
        return Blockchain.getTransactionTimestamp();
    }
}
