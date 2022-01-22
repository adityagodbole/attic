#include <iostream>
#include <concepts>
#include <vector>
#include <mutex>
#include <nlohmann/json.hpp>
#include <chrono>
#include "lib/f/f.h"

using namespace std;

class Person {
    public:
    Person() : alive{true} {}
    bool is_alive() const { return alive;}
    void set_alive(bool a) { alive = a;}

    private:
    bool alive;
};

class IndexedPerson : private Person {
    public:
    IndexedPerson(uint i): index{i}{}
    uint getIndex() const { return index; }
    using Person::is_alive;
    using Person::set_alive;

    private:
    uint index;
};

template <typename T>
concept Killable = requires(T a) {
    {a.is_alive()} -> same_as<bool>;
    {a.set_alive(bool{})} -> same_as<void>;
    {kill(a)};
};

void kill(auto &p) { // Zero runtime overhead
    p.set_alive(false);
}

template<Killable T> class Circle {
    vector<unique_ptr<T>> items;
    mutable mutex lock;

    public:

    Circle(vector<unique_ptr<T>> v) {
        items = move(v);
    }

    auto begin() const { return items.begin(); }

    auto next_alive(auto pos) const {
        lock_guard<mutex> guard(lock);
        auto it = pos;
        while(true) {
            it++; // Increment
            if(it == items.end()) { //Wrap around
                it = items.begin();
            }
            if((*it)->is_alive()) // Found alive
                break;
        }
        return it;
    }
};

int josephus(int n) {
    vector<unique_ptr<IndexedPerson>> vec;
    for(int i = 0; i < n; i++){
        vec.push_back(make_unique<IndexedPerson>(i));
    }
    // pass it to a circle
    auto circle = Circle<IndexedPerson>(move(vec));
    int aliveCount = n;
    auto pos = circle.begin();
    while(aliveCount > 1) {
        pos = circle.next_alive(pos);
        kill(**pos);
        aliveCount--;
        pos = circle.next_alive(pos);
    }
    return (*pos)->getIndex();
}
















using namespace nlohmann;
using namespace std::chrono;

int main() {
		putout(std::cout);
    auto d1 = 2019y/8/28;
    auto start = high_resolution_clock::now();
    int ans;
    for(int i = 0; i < 100; i++)
        ans = josephus(1000);
    auto end = high_resolution_clock::now();
    auto dur = duration_cast<milliseconds>(end - start);
    std::cout << json{
        {"josephus", ans},
        {"time", dur.count()}
    };
}















