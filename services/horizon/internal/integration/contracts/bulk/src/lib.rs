#![no_std]
use soroban_sdk::{contract, contractimpl, token, Address, Env, Vec};

#[contract]
pub struct Contract;

#[contractimpl]
impl Contract {
    pub fn bulk_transfer(env: Env, sender: Address, contract: Address, recipients: Vec<Address>, amounts: Vec<i128>) {
        if recipients.len() != amounts.len() {
            panic!("number of recipients does not match amounts");
        }
        sender.require_auth();
        let client = token::Client::new(&env, &contract);
        for (dest, amt) in recipients.iter().zip(amounts.iter()) {
            client.transfer(&sender, &dest, &amt);
        }
    }
}