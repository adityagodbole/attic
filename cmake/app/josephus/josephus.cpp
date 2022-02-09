
#if 0
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
#define _mut_

int main() {
	putout(_mut_ std::cout);
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
#endif

// Type your code here, or load an example.
#include "support.h"
using namespace std;


// We put all types in their own namespace
// This name space is for an entity called foo
namespace foo {
    // if we derive from `large_struct` copying of the struct
    // is disabled. You can only move the struct.
    // A struct automatically becomes non-copyable
    // if any of its data members are non-copyable
    // eg. a unique_ptr as shown below
    struct foo: private structs::heavy {
        int size;
        // We create an un-named struct called `_` to store
        // private data. This is encapsulation by convention.
        // Users of the struct are expected to not access 
        // data inside `_`
        struct {
        // A unique_ptr is an object that wraps around a pointer.
        // When this object is destroyed, the memory allocated to
        // the pointer is automatically deleted. The unique_ptr "owns"
        // the wrapped pointer.
        // A unique_ptr cannot be copied (otherwise which copy should
        // own and free the wrapped pointer?). It can be "moved" to
        // another owner.
            unique_ptr<string> x;
        } _;
    };

    void set_x(foo f, string i) {
        // easier to work with - take a ref of the private data struct
        // a ref is just another local name for the object being referred to
        // It is not a separate variable like a (raw) pointer.
        auto& priv = f._;
        priv.x = make_unique<string>(i);
    }

    // The "constructor". We do not use C++ constructors for the following
    // reason:
    // Entities have private data to enforce structural integrity.
    // What happens if a situation arises in a constructor where integrity
    // is compromised. It is not possible to return an error from a constructor
    // One could return an exception, but we avoid exceptions. (See note below
    // on exceptions)
    // The constructor returns 2 values:
    // 1. The foo object constructed
    // 2. A possible error value. The error can be a nullptr if there is no error.
    tuple<foo, err::Err> make(string x, bool err) {
        foo f;
        f._.x = make_unique<string>(x); // `_` is the private data field.
        // `make_err` creates an error. `TraceMsg` is a macro that adds line information.
        auto error = (err ? err::make_err(err::TraceMsg("inner")) : nullptr);
        // We use the initializer list syntax of c++ to create the return tuple
        // Notice that foo has been derived from `large_struct` so we cannot copy
        // it. We have to move it into the tuple.
        // Also, if you are returning a variable directly in the return, C++
        // guarantees that all objects are "moved" out.
        return {move(f), nullptr};
    }
}

int main()
{
    // Use the structured binding syntax of c++ to get both variables
    // This form creates a new variable for the error. It makes it impossible
    // to ignore the error. The programmer must make a conscious choice to
    // use it, even if it is for explicitly ignoring it.
    // entity namespaces should not be included with `using`. Explicitly
    // write foo::make
    auto [f, err] = foo::make("hello", true);
    if(err) {
        // We can wrap one error inside another, using the overloaded `make_err` function
        // so as to propagate the full information upward
        // `wr` (recursively) wraps the error given as the first argument.
        auto wr = err::make_err(err, err::TraceMsg("wrapping"));
        cerr << wr;
        return 1;
    }

    // Finally, DEFER statements use RAII to cleanup variables with "commit" symantics
    // When the function ends, the lambda passed to the final_action is run 
    // (even when an exception is thrown later in the function)

    DEFER(rollback, [&] {cerr << "Finally!";});
    /*
    ... any amount of code can go here
    */
   // The above code suceeded. We don't want to rollback
   rollback.cancel();
    return 0;
}