#ifndef __SUPPORT_H__
#define __SUPPORT_H__

#include <sstream>
#include <iostream>
#include <memory>
#include <utility>
#include <string>

namespace scope {
    // ScopeGuards
    #if 0
    struct ScopeGuardBase {
        ScopeGuardBase():
            mActive(true)
        { }

        ScopeGuardBase(ScopeGuardBase&& rhs):
            mActive(rhs.mActive)
        { rhs.dismiss(); }

        void dismiss() noexcept
        { mActive = false; }

    protected:
        ~ScopeGuardBase() = default;
        bool mActive;
    };
    #endif
    
    template<class Fun>
    struct ScopeGuard {
        ScopeGuard() = delete;
        ScopeGuard(const ScopeGuard&) = delete;

        ScopeGuard(Fun f) noexcept:
//            ScopeGuardBase(),
            mF(std::move(f))
            { }

        ScopeGuard(ScopeGuard&& rhs) noexcept :
//            ScopeGuardBase(std::move(rhs)),
            mF(std::move(rhs.mF))
            { }
        
        void cancel() noexcept
        { mActive = false; }

        ~ScopeGuard() noexcept {
            if (mActive) {
                try { mF(); } catch(...) {}
            }
        }

        ScopeGuard& operator=(const ScopeGuard&) = delete;

    private:
        Fun mF;
        bool mActive = true;
    };

    template<class Fun> ScopeGuard<Fun> guard(Fun f) {
        return ScopeGuard<Fun>(std::move(f));
    }
    #define DEFER(name,f) auto name = scope::guard(f)
// End scopeguards
}

namespace err{
    using namespace std;
    namespace priv{
        using cstr = const char * const;
        static constexpr cstr past_last_slash(cstr str, cstr last_slash)
        {
            return
                *str == '\0' ? last_slash :
                *str == '/'  ? past_last_slash(str + 1, str + 1) :
                            past_last_slash(str + 1, last_slash);
        }

        static constexpr cstr past_last_slash(cstr str) 
        { 
            return past_last_slash(str, str);
        }

        template <typename ...Args>
        string make_trace_msg(int line, const char* fileName, Args&& ...args)
        {
            std::ostringstream stream;
            stream << past_last_slash(fileName) << "(" << line << ") : ";
            (stream << ... << std::forward<Args>(args));

            return stream.str();
        }
    }
    struct _Err;
    typedef shared_ptr<_Err> Err;
    struct _Err {
        string err;
        Err parent = nullptr;
        _Err(string m) : err{m} {};
        _Err(Err p, string m) : err{m}, parent{move(p)} {};
    };
    static Err make_err(string m) {
        return make_shared<_Err>(m);
    }
    static Err make_err(Err p, string m) { 
        return std::make_shared<_Err>(move(p), m);
    }
    #define TraceMsg(...) priv::make_trace_msg(__LINE__, __FILE__, __VA_ARGS__)



} // end namespace errors

namespace structs {
    struct heavy{
        heavy(const heavy&) = delete;
        heavy& operator=(const heavy&) = delete;
        heavy() = default;
        heavy(heavy &&) = default;
        heavy &operator=(heavy &&a) = default;
    };
}

namespace std {
    string to_string(const err::Err& e) {
        string out(e->err + "\n");
        uint level = 1;
        auto parent = e->parent;
        while(parent) {
            out.append(string(level, '>') + " " + string(parent->err) + "\n");
            parent = parent->parent;
            level++;
        }
        return out;
    }

    ostream& operator<<(ostream& o, const err::Err& e){
        o << to_string(e);
        return o;
    }
}

#endif