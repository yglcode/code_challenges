package app.server;

import java.util.concurrent.atomic.AtomicIntegerArray;

public class AtomicBitSet {
	private final AtomicIntegerArray array;

	public AtomicBitSet(int length) {
		int intLen = (length + 31) / 32;
		array = new AtomicIntegerArray(intLen);
	}

	public boolean set(long n) {
		int bit = 1 << n;
		int idx = (int) (n >>> 5);
		while (true) {
			int num = array.get(idx);
			int num2 = num | bit;
			if (num == num2) {
				return false; // already set, probably by other
			}
			if (array.compareAndSet(idx, num, num2)) {
				return true; // i set it
			}
		}
	}

	public boolean get(long n) {
		int bit = 1 << n;
		int idx = (int) (n >>> 5);
		return (bit & array.get(idx)) != 0;
	}
}
