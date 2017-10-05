ExpireMap: Another Interview Code Challenge
===========================================

A variant of a map that supports the following interface. The interface allows the adding of a <key, value> pair and the removal of a key just like a regular map. The only difference is that the added entry needs to be removed after a specified timeout, if not already explicitly removed.

	interface ExpireMap<K,V> {
		// If there is no entry with the key in the map, add the key/value pair as a new entry.
		// If there is an existing entry with the key, the current entry will be replaced with the new key/value pair.
		// If the newly added entry is not removed after timeoutMs since it's added to the map, remove it. 
		void put(K key, V value,long timeoutMs);
		// Get the value associated with the key if present; otherwise, return null. 
		V get(K key);
		// Remove the entry associated with key, if any. 
		void remove(K key);
	}

Requirements:

1. The ExpireMap interface may be called concurrently by multiple threads.
2. ExpireMap should only take space proportional to the current number of entries in it.
3. The timeout should be enforced as accurately as the underlying operating system allows.
4. Try to be efficient in the big O time of each of three methods in the interface.
