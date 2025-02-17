#![no_std]
use soroban_sdk::{contract, contractimpl, log, Env, Symbol, symbol_short};

const COUNTER: Symbol = symbol_short!("COUNTER");

#[contract]
pub struct IncrementContract;

#[contractimpl]
impl IncrementContract {
    /// Increment increments an internal counter, and returns the value.
    pub fn increment(env: Env) -> u32 {
        let mut count: u32 = 0;

        // Get the current count.
        if env.storage().persistent().has(&COUNTER) {
            count = env
                .storage()
                .persistent()
                .get(&COUNTER)
                .unwrap(); // Panic if the value of COUNTER is not u32.
        }
        log!(&env, "count: {}", count);


        // Increment the count.
        count += 1;

        // Save the count.
        env.storage().persistent().set(&COUNTER, &count);

        // Return the count to the caller.
        count
    }
}

mod test;
